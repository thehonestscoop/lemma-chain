package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dgraph-io/dgo/protos/api"
	"github.com/labstack/echo"
	"github.com/speps/go-hashids"
)

type ref struct {
	Owner  *string  `json:"owner" form:"owner"`   // Optional
	Parent []string `json:"parent" form:"parent"` // Optional with facets
	Data   *string  `json:"data" form:"data"`     // Required <- check max size
}

func createNodeHandler(c echo.Context) error {

	ctx := c.Request().Context()

	r := new(ref)
	if err := c.Bind(r); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorFmt(err))
	}

	type owner struct {
		ID    string `json:"uid,omitempty"`
		Facet string `json:"node.parent|facet,omitempty"`
	}

	if r.Data == nil {
		return c.JSON(http.StatusBadRequest, ErrorFmt("data payload must not be empty"))
	} else {
		x := map[string]interface{}{}
		err := json.Unmarshal([]byte(*r.Data), &x)
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorFmt("data payload must be valid json object"))
		}

		// Check if payload size is too big
		if len([]byte(*r.Data)) > maxDataPayload*1024 {
			return c.JSON(http.StatusBadRequest, ErrorFmt(fmt.Sprintf("data payload must be less than %dkB", maxDataPayload)))
		}
	}

	txn := dg.NewTxn()
	defer txn.Discard(ctx)

	// If the owner name is supplied, check if it is the same as the logged in user.
	if r.Owner != nil {
		suppliedOwnerName := strings.TrimPrefix(strings.TrimSpace(*r.Owner), "@")
		if suppliedOwnerName == "" {
			return c.JSON(http.StatusBadRequest, ErrorFmt("owner is invalid"))
		}

		loggedInUser := c.Get("logged-in-user")
		if loggedInUser == nil || loggedInUser.(string) != suppliedOwnerName {
			return c.JSON(http.StatusUnauthorized, ErrorFmt("owner requires login"))
		}
	}

	// Convert Parents to uid
	links := []owner{}

	if len(r.Parent) != 0 {

		if len(r.Parent) > 100 {
			return c.JSON(http.StatusBadRequest, ErrorFmt("max 100 parents permitted"))
		}

		type P struct {
			facet     string
			ownerName *string
			hashID    string
		}

		Ps := []P{}

		filterQ := []string{}

		for _, val := range r.Parent {
			facet, ownerName, hashID, err := splitRefName(val)
			if err != nil {
				return c.JSON(http.StatusBadRequest, ErrorFmt(err))
			}

			Ps = append(Ps, P{facet, ownerName, hashID})

			// WARNING: SQL Injection issues
			filterQ = append(filterQ, fmt.Sprintf("( eq(node.hashid, \"%s\") )", hashID))
		}

		// Fetch all hashids and owner names using only hashids
		const q = `
		{
			find_nodes(func: has(node)) @filter( %s ) @normalize {
				uid
				hashid: node.hashid
				node.owner  {
					owner_name: user.name
				}
			}
		}
		`

		resp, err := txn.Query(ctx, fmt.Sprintf(q, strings.Join(filterQ, " OR ")))
		if err != nil {
			log.Println(err)
			return c.JSON(http.StatusInternalServerError, ErrorFmt("something went wrong. Try again"))
		}

		type Root struct {
			FindNodes []struct {
				UID       string  `json:"uid"`
				HashID    string  `json:"hashid"`
				OwnerName *string `json:"owner_name"`
			} `json:"find_nodes"`
		}

		var root Root
		err = json.Unmarshal(resp.Json, &root)
		if err != nil {
			log.Println(err)
			return c.JSON(http.StatusInternalServerError, ErrorFmt("something went wrong. Try again"))
		}

		// Validate all Parents
		rootKey := map[string]struct { // key = hashid
			UID       string  "json:\"uid\""
			HashID    string  "json:\"hashid\""
			OwnerName *string "json:\"owner_name\""
		}{}
		for j, n := range root.FindNodes {
			rootKey[n.HashID] = root.FindNodes[j]
		}

		for _, p := range Ps {
			userHashID := p.hashID

			// Check if hashID is valid. Does it exist?
			rk, exists := rootKey[userHashID]
			if !exists {
				return c.JSON(http.StatusBadRequest, ErrorFmt("provided parent does not exist"))
			}

			// Check if node's owner is consistent with what user provided
			if p.ownerName == nil {
				if rk.OwnerName != nil {
					return c.JSON(http.StatusBadRequest, ErrorFmt("provided parent does not exist"))
				}
			} else {
				if rk.OwnerName == nil {
					return c.JSON(http.StatusBadRequest, ErrorFmt("provided parent does not exist"))
				} else if *p.ownerName != *rk.OwnerName {
					return c.JSON(http.StatusBadRequest, ErrorFmt("provided parent does not exist"))
				}
			}

			links = append(links, owner{rk.UID, p.facet})
		}
	}

	// Attempt to save ref

	compactedJson, _ := compactJson(*r.Data)

	data := map[string]interface{}{
		"node":            true,
		"node.xdata":      compactedJson,
		"node.searchable": false,
		"node.created_at": time.Now(),
	}

	if r.Owner != nil {
		// an owner has been provided and it is validated
		data["node.owner"] = &owner{ID: c.Get("logged-in-user-uid").(string)}
	}

	if len(links) > 0 {
		data["node.parent"] = links
	}

	assigned, err := txn.Mutate(ctx, &api.Mutation{SetJson: marshall(data)})
	if err != nil {
		log.Println(err)
		return c.JSON(http.StatusInternalServerError, ErrorFmt("something went wrong. Try again"))
	}

	// Update hashid of link
	uid := assigned.Uids["blank-0"]

	hd := hashids.NewData()
	hd.Salt = hashIDSalt
	hd.MinLength = 6
	hd.Alphabet = "abcdefghijklmnopqrstuvwxyz1234567890"
	h, _ := hashids.NewWithData(hd)
	hashid, err := h.EncodeHex(uid[2:])
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorFmt("something went wrong. Try again"))
	}

	_, err = txn.Mutate(ctx, &api.Mutation{SetJson: marshall(map[string]interface{}{
		"uid":         uid,
		"node.hashid": hashid,
	})})
	if err != nil {
		log.Println(err)
		return c.JSON(http.StatusInternalServerError, ErrorFmt("something went wrong. Try again"))
	}

	linkAddress := hashid
	if r.Owner != nil {
		// an owner has been provided and it is validated
		linkAddress = "@" + c.Get("logged-in-user").(string) + "/" + linkAddress
	}

	err = txn.Commit(ctx)
	if err != nil {
		log.Println(err)
		return c.JSON(http.StatusInternalServerError, ErrorFmt("something went wrong. Try again"))
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"link": linkAddress,
	})
}

// splitRefName returns facet, owner name and hashid
func splitRefName(refName string) (string, *string, string, error) {

	refName = strings.TrimSpace(refName)
	if refName == "" {
		return "", nil, "", errors.New("invalid parent")
	}

	// Obtain facet name
	splits := strings.Split(refName, ":")
	if len(splits) == 0 || len(splits) == 1 {
		return "", nil, "", errors.New("invalid parent")
	}

	facet := splits[0]
	if facet == "" {
		return "", nil, "", errors.New("invalid parent: ref type must not be empty")
	}
	if len(facet) > 30 {
		return "", nil, "", errors.New("invalid parent: ref type must be at most 30 characters")
	}

	remainder := strings.Join(splits[1:], ":")

	if len(remainder) == 0 {
		return "", nil, "", errors.New("invalid parent") // Nothing after :
	}

	// Obtain owner name and hashid
	splits = strings.Split(remainder, "/")

	// Check if it contains a "/"
	switch len(splits) {
	case 1:
		// Only hashid is present
		return facet, nil, splits[0], nil
	case 2:
		if splits[0] == "" {
			return "", nil, "", errors.New("invalid parent")
		}
	}

	if string(splits[0][0]) != "@" {
		// First character of owner name must contain @
		return "", nil, "", errors.New("provided parent does not exist")
	}

	ownerName := strings.TrimSpace(strings.TrimPrefix(splits[0], "@"))
	if ownerName == "" {
		return "", nil, "", errors.New("invalid parent")
	}

	hashid := strings.Join(splits[1:], "/")
	if hashid == "" {
		return "", nil, "", errors.New("invalid parent")
	}

	return facet, &ownerName, hashid, nil
}
