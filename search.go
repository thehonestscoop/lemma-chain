// Copyright 2019
// The Honest Scoop and P.J. Siripala
// All rights reserved

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo"
	"github.com/patrickmn/go-cache"
)

type searchRef struct {
	Name           *string   `json:"name"`
	ID             string    `json:"id"`
	Data           string    `json:"data"`
	SearchTitle    *string   `json:"search_title"`    // add omitempty
	SearchSynopsis *string   `json:"search_synopsis"` // add omitempty
	CreatedAt      time.Time `json:"created_at"`
}

func (s *searchRef) MarshalJSON() ([]byte, error) {

	data := map[string]interface{}{}

	err := json.Unmarshal([]byte(s.Data), &data)
	if err != nil {
		return nil, err
	}

	out := map[string]interface{}{
		"data":            data,
		"created_at":      s.CreatedAt,
		"search_title":    s.SearchTitle,
		"search_synopsis": s.SearchSynopsis,
	}

	if s.Name == nil {
		out["id"] = s.ID
	} else {
		out["id"] = "@" + *s.Name + "/" + s.ID
	}

	return json.Marshal(out)
}

func searchHandler(c echo.Context) error {
	ctx := c.Request().Context()

	terms := strings.TrimSpace(c.Param("terms"))
	if terms == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{})
	}

	// Check cache
	key := fmt.Sprintf("search-%s", terms)
	cachedData, found := memoryCache.Get(key)
	if found {
		log.Println("Using cache:" + key)
		return c.JSON(http.StatusOK, cachedData)
	}

	txn := dg.NewReadOnlyTxn()

	vars := map[string]string{
		"$terms": terms,
	}

	q := `
		query withvar($terms: string) {
			results(func: eq(node.searchable, true), orderdesc: node.created_at) @normalize @filter(allofterms(node.search_title, $terms) OR alloftext(node.search_synopsis, $terms)) {
				node.owner {
					name: user.name
				}
				id: node.hashid
				data: node.xdata
				search_title: node.search_title
				search_synopsis: node.search_synopsis
				created_at: node.created_at
			}
		}
	`

	if stdQueryTimeout != 0 {
		// Create a max query timeout
		_ctx, cancel := context.WithTimeout(ctx, time.Duration(stdQueryTimeout)*time.Millisecond)
		defer cancel()
		ctx = _ctx
	}

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
		Results []searchRef `json:"results"`
	}

	var root Root
	err = json.Unmarshal(resp.Json, &root)
	if err != nil {
		log.Println(err)
		return c.JSON(http.StatusInternalServerError, ErrorFmt("something went wrong. Try again"))
	}

	// Store data in cache
	memoryCache.Set(key, root, cache.DefaultExpiration)

	return c.JSON(http.StatusOK, root)
}
