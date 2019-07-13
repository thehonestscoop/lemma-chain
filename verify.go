// Copyright 2019
// The Honest Scoop and P.J. Siripala
// All rights reserved

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/xerrors"
	"hash/crc32"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dgraph-io/dgo/protos/api"
	"github.com/labstack/echo"
	"github.com/myesui/uuid"
	"gopkg.in/gomail.v2"
)

func init() {
	go func() {
		c := time.Tick(24 * time.Hour) // Run daily
		for range c {
			cleanup(context.Background())
		}
	}()
}

// cleanup will remove all nodes that have not been activated for 48 hours.
func cleanup(ctx context.Context) {

	txn := dg.NewTxn()
	defer txn.Discard(ctx)

	vars := map[string]string{
		"$dt": time.Now().UTC().Add(-2 * 24 * time.Hour).Format(time.RFC3339),
	}

	const q = `
		query withvar($dt: string) {
			find_users(func: eq(user.validated, false)) @filter(le(user.created_at, $dt))
			{
				uid
			}
		}
	`

	resp, err := txn.QueryWithVars(ctx, q, vars)
	if err != nil {
		log.Println(xerrors.Errorf("A: %w", err))
		return
	}

	type Root struct {
		FindUsers []struct {
			UID string `json:"uid"`
		} `json:"find_users"`
	}

	var r Root
	err = json.Unmarshal(resp.Json, &r)
	if err != nil {
		log.Println(xerrors.Errorf("B: %w", err))
		return
	}

	if len(r.FindUsers) == 0 {
		return
	}

	// Delete unvalidated accounts
	_, err = txn.Mutate(ctx, &api.Mutation{DeleteJson: marshal(r.FindUsers)})
	if err != nil {
		log.Println(xerrors.Errorf("C: %w", err))
		return
	}

	err = txn.Commit(ctx)
	if err != nil {
		log.Println(xerrors.Errorf("D: %w", err))
		return
	}

}

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
	m.SetBody("text/html", "Click on the link within 48 hours to activate account: "+fmt.Sprintf("<a href=\"%s\">%s</a>", url, url))

	d := gomail.NewDialer(smtpHost, smtpPort, gmailAccount, gmailPassword)

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
