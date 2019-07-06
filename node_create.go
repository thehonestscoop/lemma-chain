// Copyright 2019
// The Honest Scoop and P.J. Siripala
// All rights reserved

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

var h *hashids.HashID

func init() {
	// Configure hashid generator for converting uids to hashids
	hd := hashids.NewData()
	hd.Salt = hashIDSalt
	hd.MinLength = 6
	hd.Alphabet = "abcdefghijklmnpqrstuvwxyz123456789" // Remove o and 0 to avoid confusion
	h, _ = hashids.NewWithData(hd)
}

type ref struct {
	Owner          *string  `json:"owner" form:"owner"`                     // Optional
	Parents        []string `json:"parents" form:"parents"`                 // Optional with facets
	Data           *string  `json:"data" form:"data"`                       // Required <- check max size
	Searchable     bool     `json:"searchable" form:"searchable"`           // Defaults to false
	SearchTitle    *string  `json:"search_title" form:"search_title"`       // Optional
	SearchSynopsis *string  `json:"search_synopsis" form:"search_synopsis"` // Optional
	RecaptchaCode  string   `json:"recaptcha_code" form:"recaptcha_code"`   // Required
}

// createNodeHandler is the handler to create a ref.
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

	// Validate search related input
	if r.Searchable == true {
		// We require at least a title or synopsis
		if r.SearchTitle == nil && r.SearchSynopsis == nil {
			return c.JSON(http.StatusBadRequest, ErrorFmt("when searchable is true, a search title or search synopsis is required"))
		}
	}

	if r.SearchTitle != nil {
		*r.SearchTitle = strings.TrimSpace(*r.SearchTitle)

		if *r.SearchTitle == "" {
			return c.JSON(http.StatusBadRequest, ErrorFmt("search title must not be empty"))
		}

		if len(*r.SearchTitle) > 100 {
			return c.JSON(http.StatusBadRequest, ErrorFmt("search title must be less than 100 characters"))
		}
	}

	if r.SearchSynopsis != nil {
		*r.SearchSynopsis = strings.TrimSpace(*r.SearchSynopsis)

		if *r.SearchSynopsis == "" {
			return c.JSON(http.StatusBadRequest, ErrorFmt("search synopsis must not be empty"))
		}

		if len(*r.SearchSynopsis) > 800 {
			return c.JSON(http.StatusBadRequest, ErrorFmt("search synopsis must be less than 800 characters"))
		}
	}

	err := recaptchaCheck(r.RecaptchaCode)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorFmt("recaptcha invalid"))
	}

	txn := dg.NewTxn()
	defer txn.Discard(ctx)

	// If the owner name is supplied, check if it is the same as the logged in user.
	if r.Owner != nil {
		suppliedOwnerName := strings.TrimPrefix(strings.TrimSpace(*r.Owner), "@")
		if suppliedOwnerName == "" {
			return c.JSON(http.StatusBadRequest, ErrorFmt("owner is invalid"))
		}

		// suppliedOwnerName could be account name or account email
		loggedInUser := c.Get("logged-in-user")
		loggedInUserEmail := c.Get("logged-in-user-email")
		if loggedInUser == nil || ((loggedInUser.(string) != suppliedOwnerName) && (loggedInUserEmail.(string) != suppliedOwnerName)) {
			return c.JSON(http.StatusUnauthorized, ErrorFmt("owner requires login"))
		}
	}

	// Convert Parents to uid
	links := []owner{}

	if len(r.Parents) != 0 {

		if len(r.Parents) > 250 {
			return c.JSON(http.StatusBadRequest, ErrorFmt("max 250 parent refs permitted"))
		}

		type P struct {
			facet     string
			ownerName *string
			hashID    string
		}

		Ps := []P{}

		hashids := []string{}

		for _, val := range r.Parents {
			facet, ownerName, hashID, err := splitRefName(val)
			if err != nil {
				return c.JSON(http.StatusBadRequest, ErrorFmt(err))
			}

			Ps = append(Ps, P{facet, ownerName, hashID})

			// WARNING: SQL Injection issues
			hashids = append(hashids, fmt.Sprintf("\"%s\"", hashID))
		}

		// Fetch all hashids and owner names using only hashids
		const q = `
			{
				find_nodes(func: eq(node.hashid, %s)) @normalize {
					uid
					hashid: node.hashid
					node.owner  {
						owner_name: user.name
					}
				}
			}
		`

		resp, err := txn.Query(ctx, fmt.Sprintf(q, "["+strings.Join(hashids, ", ")+"]"))
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
				return c.JSON(http.StatusBadRequest, ErrorFmt("provided parent ref does not exist"))
			}

			// Check if node's owner is consistent with what user provided
			if p.ownerName == nil {
				if rk.OwnerName != nil {
					return c.JSON(http.StatusBadRequest, ErrorFmt("provided parent ref does not exist"))
				}
			} else {
				if rk.OwnerName == nil {
					return c.JSON(http.StatusBadRequest, ErrorFmt("provided parent ref does not exist"))
				} else if *p.ownerName != *rk.OwnerName {
					return c.JSON(http.StatusBadRequest, ErrorFmt("provided parent ref does not exist"))
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
		"node.searchable": r.Searchable,
		"node.created_at": time.Now(),
	}

	if r.Owner != nil {
		// an owner has been provided and it is validated
		data["node.owner"] = &owner{ID: c.Get("logged-in-user-uid").(string)}
	}

	if len(links) > 0 {
		data["node.parent"] = links
	}

	if r.SearchTitle != nil {
		data["node.search_title"] = *r.SearchTitle
	}

	if r.SearchSynopsis != nil {
		data["node.search_synopsis"] = *r.SearchSynopsis
	}

	assigned, err := txn.Mutate(ctx, &api.Mutation{SetJson: marshal(data)})
	if err != nil {
		log.Println(err)
		return c.JSON(http.StatusInternalServerError, ErrorFmt("something went wrong. Try again"))
	}

	// Update hashid of link
	uid := assigned.Uids["blank-0"]

	hashid, err := h.EncodeHex(uid[2:])
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorFmt("something went wrong. Try again"))
	}

	_, err = txn.Mutate(ctx, &api.Mutation{SetJson: marshal(map[string]interface{}{
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
		return "", nil, "", errors.New("invalid parent ref")
	}

	// Obtain facet name
	splits := strings.Split(refName, ":")
	if len(splits) == 0 || len(splits) == 1 {
		return "", nil, "", errors.New("invalid parent ref")
	}

	facet := splits[0]
	facet = strings.TrimSpace(facet)
	if facet == "" {
		return "", nil, "", errors.New("invalid parent: ref type must not be empty")
	}
	if len(facet) > 75 {
		return "", nil, "", errors.New("invalid parent: ref type must be at most 75 characters")
	}

	for _, char := range facet {
		if char == 34 || char == 64 || char == 47 {
			return "", nil, "", errors.New("invalid parent: ref type must not contain \", @ or /")
		}
	}

	remainder := strings.Join(splits[1:], ":")

	if len(remainder) == 0 {
		return "", nil, "", errors.New("invalid parent ref") // Nothing after :
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
			return "", nil, "", errors.New("invalid parent ref")
		}
	}

	if string(splits[0][0]) != "@" {
		// First character of owner name must contain @
		return "", nil, "", errors.New("provided parent ref does not exist")
	}

	ownerName := strings.TrimSpace(strings.TrimPrefix(splits[0], "@"))
	if ownerName == "" {
		return "", nil, "", errors.New("invalid parent ref")
	}

	hashid := strings.Join(splits[1:], "/")
	if hashid == "" {
		return "", nil, "", errors.New("invalid parent ref")
	}

	return facet, &ownerName, hashid, nil
}
