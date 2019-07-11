// Copyright 2019
// The Honest Scoop and P.J. Siripala
// All rights reserved

package main

import (
	"encoding/json"
	"fmt"
	"hash/crc32"
	"log"
	"net/http"
	"strings"

	"github.com/dgraph-io/dgo/protos/api"
	"github.com/labstack/echo"
	"github.com/myesui/uuid"
	"gopkg.in/gomail.v2"
)

func sendEmail(email, code string) error {

	gmailAccount := gmailAccount
	if !strings.Contains(gmailAccount, "@") {
		gmailAccount = gmailAccount + "@gmail.com"
	}

	url := fmt.Sprintf("%s/verify/%s", serverHostUrl, code)

	m := gomail.NewMessage()
	m.SetHeader("From", gmailAccount)
	m.SetHeader("To", email)
	m.SetHeader("Subject", "Activate Lemma Chain Account")
	m.SetBody("text/html", "Click on the link to activate account: "+fmt.Sprintf("<a href=\"%s\">%s</a>", url, url))

	d := gomail.NewDialer("smtp.gmail.com", 587, gmailAccount, gmailPassword)

	// Send the email to Bob, Cora and Dan.
	err := d.DialAndSend(m)
	return err
}

// verifyHandler is used to verify account creation
func verifyHandler(c echo.Context) error {
	ctx := c.Request().Context()

	code := strings.TrimSpace(c.Param("code"))
	if code == "" {
		return c.Redirect(http.StatusTemporaryRedirect, website)
	}

	// Find node with code
	txn := dg.NewTxn()
	defer txn.Discard(ctx)

	vars := map[string]string{
		"$code": code,
	}

	q := `
		query withvar($code: string) {
			nodes(func: eq(user.code, $code))  {
				uid
			}
		}
	`

	resp, err := txn.QueryWithVars(ctx, q, vars)
	if err != nil {
		log.Println(err)
		return c.JSONPretty(http.StatusInternalServerError, ErrorFmt("something went wrong. Try again"), "  ")
	}

	type Root struct {
		Nodes []struct {
			UID string `json:"uid"`
		} `json:"nodes"`
	}

	var root Root
	err = json.Unmarshal(resp.Json, &root)
	if err != nil {
		log.Println(err)
		return c.JSON(http.StatusInternalServerError, ErrorFmt("something went wrong. Try again"))
	}

	if len(root.Nodes) == 0 {
		return c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s?activated=0", website))
	}

	// Update node as active
	data := struct {
		UID       string `json:"uid"`
		Code      string `json:"user.code"`
		Validated bool   `json:"user.validated"`
	}{
		root.Nodes[0].UID,
		fmt.Sprintf("%08x", crc32.ChecksumIEEE([]byte(uuid.NewV4().String()))),
		true,
	}

	_, err = txn.Mutate(ctx, &api.Mutation{SetJson: marshal(data)})
	if err != nil {
		log.Println(err)
		return c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s?activated=0", website))
	}

	return c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s?activated=1", website))
}
