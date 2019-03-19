package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/labstack/echo"
)

func loginChecker(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {

		ctx := c.Request().Context()

		account := strings.TrimSpace(c.Request().Header.Get("X-AUTH-ACCOUNT")) // Can be an account name or email
		password := strings.TrimSpace(c.Request().Header.Get("X-AUTH-PASSWORD"))
		name := strings.TrimPrefix(account, "@")
		email := strings.ToLower(account)

		if account == "" || password == "" {
			return next(c)
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
					checkpwd(user.password, $password)
				}

				user_check2(func: eq(user.name, $name), first: 1) {
					uid
					user.name
					checkpwd(user.password, $password)
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
				UID      string `json:"uid"`
				Name     string `json:"user.name"`
				Checkpwd bool   `json:"checkpwd(user.password)"`
			} `json:"user_check1"`
			Check2 []struct {
				UID      string `json:"uid"`
				Name     string `json:"user.name"`
				Checkpwd bool   `json:"checkpwd(user.password)"`
			} `json:"user_check2"`
		}

		var r Root
		err = json.Unmarshal(resp.Json, &r)
		if err != nil {
			log.Println(err)
			return c.JSON(http.StatusInternalServerError, ErrorFmt("something went wrong. Try again"))
		}

		if len(r.Check1) == 1 && r.Check1[0].Checkpwd == true {
			c.Set("logged-in-user", r.Check1[0].Name)
			c.Set("logged-in-user-uid", r.Check1[0].UID)
		} else if len(r.Check2) == 1 && r.Check2[0].Checkpwd == true {
			c.Set("logged-in-user", r.Check2[0].Name)
			c.Set("logged-in-user-uid", r.Check2[0].UID)
		}

		return next(c)
	}
}
