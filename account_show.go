// Copyright 2019
// The Honest Scoop and P.J. Siripala
// All rights reserved

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo"
)

type showAccountModel struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`

	Refs []struct {
		ID             string    `json:"id"`
		Data           string    `json:"data"`
		Searchable     bool      `json:"searchable"`
		SearchTitle    *string   `json:"search_title,omitempty"`
		SearchSynopsis *string   `json:"search_synopsis,omitempty"`
		CreatedAt      time.Time `json:"created_at"`
	} `json:"refs"`
}

// showAccountHandler will list account information and all refs owned by the account.
// If logged in, display email address and all nodes.
// If not logged in, hide email address and only list all searchable nodes.
func showAccountHandler(c echo.Context) error {

	ctx := c.Request().Context()

	loggedInUser := c.Get("logged-in-user")

	// Check if name has "@" prefix
	if !strings.HasPrefix(c.Param("name"), "@") {
		return c.NoContent(http.StatusNotFound)
	}
	name := strings.ToLower(strings.TrimPrefix(c.Param("name"), "@"))

	// Query for all nodes owned by user
	txn := dg.NewReadOnlyTxn()

	vars := map[string]string{
		"$name": name,
	}

	q := `
		query withvar($name: string) {
			nodes(func: eq(user.name, $name))  {
				name: user.name
				%s # email: user.email

				refs: ~node.owner(orderdesc: node.created_at) @filter( %s ) {
					# uid
					id: node.hashid
					data: node.xdata
					searchable: node.searchable
					search_title: node.search_title
					search_synopsis: node.search_synopsis
					created_at: node.created_at
				}
			}
		}
	`

	if loggedInUser != nil && loggedInUser.(string) == name {
		// User logged in
		q = fmt.Sprintf(q, "email: user.email", "")
	} else {
		// User not logged in
		q = fmt.Sprintf(q, "", "eq(node.searchable, true)")
	}

	resp, err := txn.QueryWithVars(ctx, q, vars)
	if err != nil {
		log.Println(err)
		return c.JSON(http.StatusInternalServerError, ErrorFmt("something went wrong. Try again"))
	}

	type Root struct {
		Model []showAccountModel `json:"nodes"`
	}

	var root Root
	err = json.Unmarshal(resp.Json, &root)
	if err != nil {
		log.Println(err)
		return c.JSON(http.StatusInternalServerError, ErrorFmt("something went wrong. Try again"))
	}

	if len(root.Model) == 1 {
		root.Model[0].Name = "@" + name

		if len(root.Model[0].Refs) == 0 {
			root.Model[0].Refs = []struct {
				ID             string    `json:"id"`
				Data           string    `json:"data"`
				Searchable     bool      `json:"searchable"`
				SearchTitle    *string   `json:"search_title,omitempty"`
				SearchSynopsis *string   `json:"search_synopsis,omitempty"`
				CreatedAt      time.Time `json:"created_at"`
			}{}
		} else {

			// Change the id of each ref to include the owner name
			for i := range root.Model[0].Refs {
				root.Model[0].Refs[i].ID = root.Model[0].Name + "/" + root.Model[0].Refs[i].ID
			}

		}

		return c.JSONPretty(http.StatusOK, root.Model[0], "  ")
	}

	return c.NoContent(http.StatusNotFound)
}
