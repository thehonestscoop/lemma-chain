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

	"github.com/labstack/echo"
	"github.com/patrickmn/go-cache"
)

// loginChecker is middleware that will check if the user is attempting to login
// using the request header. If login is successful, record the uid and owner name
// to echo context.
func loginChecker(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {

		ctx := c.Request().Context()

		account := strings.TrimSpace(c.Request().Header.Get("X-AUTH-ACCOUNT")) // Can be an account name or email
		password := strings.TrimSpace(c.Request().Header.Get("X-AUTH-PASSWORD"))
		name := strings.ToLower(strings.TrimPrefix(account, "@"))
		email := strings.ToLower(account)

		if account == "" || password == "" {
			return next(c)
		}

		// Check cache
		key := fmt.Sprintf("middleware.loginChecker-%s-%s", account, password)
		cachedData, found := memoryCache.Get(key)
		if found {
			// log.Println("Using cache:" + fmt.Sprintf("middleware.loginChecker-%s-xxx", account))

			cd := cachedData.(map[string]string)

			c.Set("logged-in-user", cd["user"])
			c.Set("logged-in-user-uid", cd["uid"])
			c.Set("logged-in-user-email", cd["email"])
		}

		// Check login
		txn := dg.NewReadOnlyTxn()

		vars := map[string]string{
			"$email":    email,
			"$name":     name,
			"$password": password,
		}

		const q = `
			query withvar($email: string, $name: string, $password: string) {
				user_check1(func: eq(user.email, $email), first: 1) {
					uid
					user.name
					user.email
					user.validated
					checkpwd: checkpwd(user.password, $password)
				}

				user_check2(func: eq(user.name, $name), first: 1) {
					uid
					user.name
					user.email
					user.validated
					checkpwd: checkpwd(user.password, $password)
				}
			}
		`

		resp, err := txn.QueryWithVars(ctx, q, vars)
		if err != nil {
			log.Println(err)
			return c.JSON(http.StatusInternalServerError, ErrorFmt("something went wrong. Try again"))
		}

		// Check if a user exists
		type Root struct {
			Check1 []struct {
				UID       string `json:"uid"`
				Name      string `json:"user.name"`
				Email     string `json:"user.email"`
				Validated bool   `json:"user.validated"`
				Checkpwd  bool   `json:"checkpwd"`
			} `json:"user_check1"`
			Check2 []struct {
				UID       string `json:"uid"`
				Name      string `json:"user.name"`
				Email     string `json:"user.email"`
				Validated bool   `json:"user.validated"`
				Checkpwd  bool   `json:"checkpwd"`
			} `json:"user_check2"`
		}

		var r Root
		err = json.Unmarshal(resp.Json, &r)
		if err != nil {
			log.Println(err)
			return c.JSON(http.StatusInternalServerError, ErrorFmt("something went wrong. Try again"))
		}

		if len(r.Check1) == 0 && len(r.Check2) == 0 {
			// User not found
			return c.NoContent(http.StatusUnauthorized)
		} else if len(r.Check1) == 1 {

			if r.Check1[0].Checkpwd == false {
				// Password incorrect
				return c.NoContent(http.StatusUnauthorized)
			}

			if r.Check1[0].Validated == false {
				// User has not verified email
				return c.JSON(http.StatusUnauthorized, ErrorFmt("account requires email validation"))
			}

			c.Set("logged-in-user", r.Check1[0].Name)
			c.Set("logged-in-user-uid", r.Check1[0].UID)
			c.Set("logged-in-user-email", r.Check1[0].Email)

			// Store data in cache
			memoryCache.Set(key, map[string]string{"user": r.Check1[0].Name, "uid": r.Check1[0].UID, "email": r.Check1[0].Email}, cache.DefaultExpiration)

		} else if len(r.Check2) == 1 {

			if r.Check2[0].Checkpwd == false {
				// Password incorrect
				return c.NoContent(http.StatusUnauthorized)
			}

			if r.Check2[0].Validated == false {
				// User has not verified email
				return c.JSON(http.StatusUnauthorized, ErrorFmt("account requires email verification"))
			}

			c.Set("logged-in-user", r.Check2[0].Name)
			c.Set("logged-in-user-uid", r.Check2[0].UID)
			c.Set("logged-in-user-email", r.Check2[0].Email)

			// Store data in cache
			memoryCache.Set(key, map[string]string{"user": r.Check2[0].Name, "uid": r.Check2[0].UID, "email": r.Check2[0].Email}, cache.DefaultExpiration)

		}

		return next(c)
	}
}

// nocache instructs browsers to not record response
func nocache(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Header().Set("Cache-Control", "private")
		return next(c)
	}
}
