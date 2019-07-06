// Copyright 2019
// The Honest Scoop and P.J. Siripala
// All rights reserved

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo"
	"github.com/patrickmn/go-cache"
)

type OwnerModel struct {
	Name string `json:"user.name"`
}

type ChainModel struct {
	UID     string       `json:"uid"` // Required due to: https://github.com/dgraph-io/dgraph/issues/3163
	Owner   []OwnerModel `json:"node.owner"`
	HashID  string       `json:"node.hashid"`
	XData   string       `json:"node.xdata"`
	Parents []ChainModel `json:"node.parent"`
	Facet   interface{}  `json:"node.parent|facet"` // Changed from *string due to https://github.com/dgraph-io/dgraph/issues/3582
}

func (cm *ChainModel) MarshalJSON() ([]byte, error) {

	data := map[string]interface{}{}

	err := json.Unmarshal([]byte(cm.XData), &data)
	if err != nil {
		return nil, err
	}

	out := map[string]interface{}{
		"data": data,
	}

	if len(cm.Owner) == 1 {
		out["id"] = "@" + cm.Owner[0].Name + "/" + cm.HashID
	} else {
		out["id"] = cm.HashID
	}

	if len(cm.Parents) != 0 {
		if cm.Parents[0].UID != "" {
			// https://github.com/dgraph-io/dgraph/issues/3163
			// This approach needs testing
			out["refs"] = cm.Parents
		}
	} else {
		out["refs"] = []int{}
	}

	// https://github.com/dgraph-io/dgraph/issues/3582
	if cm.Facet != nil {
		switch v := cm.Facet.(type) {
		case string:
			out["ref_type"] = v
		case []string:
			out["ref_type"] = v[0]
		case []interface{}:
			out["ref_type"] = v[0]
		}
	}

	return json.Marshal(out)
}

// findChainHandler will list all nodes linked to the provided ref.
func findChainHandler(c echo.Context) error {
	ctx := c.Request().Context()

	nodeID := strings.ToLower(strings.TrimSpace(c.Param("*")))

	ownerName, hashID, err := splitNodeID(nodeID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorFmt("can't find ref"))
	}

	// Depth can be configured by query param
	var depth int
	_depth := c.QueryParam("depth")
	if _depth != "" {
		depth, err = strconv.Atoi(_depth)
		if err != nil || depth == 0 {
			return c.JSON(http.StatusBadRequest, ErrorFmt("depth query param is malformed"))
		}
	}

	// Reference types can be configured by query param
	_refTypes := c.QueryParam("types")
	refTypes := strings.Split(_refTypes, ",")
	if len(refTypes) == 1 && refTypes[0] == "" {
		refTypes = []string{}
	}
	sort.Strings(refTypes)

	// Check cache
	key := fmt.Sprintf("*-%s-%s-%s", nodeID, _depth, strings.Join(refTypes, ","))
	cachedData, found := memoryCache.Get(key)
	if found {
		// log.Println("Using cache:" + key)
		return c.JSON(http.StatusOK, cachedData)
	}

	if stdQueryTimeout != 0 {
		// Create a max query timeout
		_ctx, cancel := context.WithTimeout(ctx, time.Duration(stdQueryTimeout)*time.Millisecond)
		defer cancel()
		ctx = _ctx
	}

	txn := dg.NewReadOnlyTxn()

	// Check if hashID is owned by owner name
	vars := map[string]string{
		"$hashid": hashID,
	}

	q := `
		query withvar($hashid: string) {
			check(func: eq(node.hashid, $hashid)) @normalize {
				uid: uid
			    node.owner {
			    	name: user.name 
			    }
			}
		}
	`

	resp, err := txn.QueryWithVars(ctx, q, vars)
	if err != nil {
		if strings.Contains(err.Error(), "context canceled") {
			return c.NoContent(http.StatusNoContent)
		} else if strings.Contains(err.Error(), "context deadline exceeded") {
			return c.NoContent(http.StatusRequestTimeout)
		}
		log.Println(err)
		return c.JSON(http.StatusInternalServerError, ErrorFmt("something went wrong. Try again"))
	}

	type Root struct {
		Check []struct {
			UID  string  `json:"uid"`
			Name *string `json:"name"`
		} `json:"check"`
	}

	var root Root
	err = json.Unmarshal(resp.Json, &root)
	if err != nil {
		log.Println(err)
		return c.JSON(http.StatusInternalServerError, ErrorFmt("something went wrong. Try again"))
	}

	if ownerName == nil {
		if !(len(root.Check) == 1 && (root.Check[0].Name == nil)) {
			// Can't find the hashid or name exists
			return c.JSON(http.StatusBadRequest, ErrorFmt("can't find ref"))
		}
	} else {
		if len(root.Check) == 0 {
			return c.JSON(http.StatusBadRequest, ErrorFmt("can't find ref"))
		}

		actualName := root.Check[0].Name

		if actualName == nil {
			return c.JSON(http.StatusBadRequest, ErrorFmt("can't find ref"))
		} else if *actualName != *ownerName {
			return c.JSON(http.StatusBadRequest, ErrorFmt("can't find ref"))
		}
	}

	// Find entire chain

	var recursive string
	if _depth == "" {
		recursive = "@recurse(loop:false)"
	} else {
		recursive = fmt.Sprintf("@recurse(depth:%d,loop:false)", depth)
	}

	facetFilters := []string{}
	var facetFiltersStr string
	if len(refTypes) > 0 {
		for _, val := range refTypes {
			facetFilters = append(facetFilters, fmt.Sprintf("eq(facet, \"%s\")", val))
		}

		facetFiltersStr = "@facets( " + strings.Join(facetFilters, " or ") + " )"
	}

	q = `
		query withvar($hashid: string) {
			chain(func: eq(node.hashid, $hashid)) %s {
				uid
				node.owner
				user.name
				node.hashid
				node.xdata
				node.parent @facets %s
			}
		}
	`

	resp, err = txn.QueryWithVars(ctx, fmt.Sprintf(q, recursive, facetFiltersStr), vars)
	if err != nil {
		if strings.Contains(err.Error(), "context canceled") {
			return c.NoContent(http.StatusNoContent)
		} else if strings.Contains(err.Error(), "context deadline exceeded") {
			return c.NoContent(http.StatusRequestTimeout)
		}
		log.Println(err)
		return c.JSON(http.StatusInternalServerError, ErrorFmt("something went wrong. Try again"))
	}

	type RootChain struct {
		Chain []*ChainModel `json:"chain"`
	}

	var rootChain RootChain
	err = json.Unmarshal(resp.Json, &rootChain)
	if err != nil {
		log.Println(err)
		return c.JSON(http.StatusInternalServerError, ErrorFmt("something went wrong. Try again"))
	}

	/////// Expand owners in deeply nested nodes due to BUG: https://github.com/dgraph-io/dgraph/issues/3634

	uidToOwnerModel := map[string]string{} // key is uid, value is owner account name

	var inspect func([]ChainModel, bool)
	inspect = func(chain []ChainModel, insert bool) {

		for i := range chain {
			cm := &chain[i]

			if !insert {
				if len(cm.Owner) > 0 {
					uidToOwnerModel[cm.UID] = cm.Owner[0].Name
				}
			} else {
				if ownerID, exists := uidToOwnerModel[cm.UID]; exists {
					cm.Owner = []OwnerModel{OwnerModel{Name: ownerID}}
				}
			}

			inspect(cm.Parents, insert)
		}
	}

	cm := rootChain.Chain[0]
	if len(cm.Owner) > 0 {
		uidToOwnerModel[cm.UID] = cm.Owner[0].Name
	}
	inspect(cm.Parents, false)

	// Insert owners back into model
	if ownerID, exists := uidToOwnerModel[cm.UID]; exists {
		cm.Owner = []OwnerModel{OwnerModel{Name: ownerID}}
	}
	inspect(cm.Parents, true)

	///////

	// Store data in cache
	memoryCache.Set(key, rootChain.Chain[0], cache.DefaultExpiration)

	return c.JSON(http.StatusOK, rootChain.Chain[0])
}

// splitNodeID returns owner name and hashid
func splitNodeID(nodeID string) (*string, string, error) {

	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return nil, "", errors.New("invalid")
	}

	// Extract owner name
	splits := strings.Split(nodeID, "/")

	if len(splits) == 1 {
		// No owner found
		return nil, splits[0], nil
	}

	if string(splits[0][0]) != "@" {
		// First character of owner name must contain @
		return nil, "", errors.New("invalid")
	}

	ownerID := strings.TrimSpace(strings.TrimPrefix(splits[0], "@"))

	if ownerID == "" {
		return nil, "", errors.New("invalid")
	}

	return &ownerID, splits[1], nil
}
