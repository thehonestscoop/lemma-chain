package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
)

func marshall(d interface{}) []byte {
	x, _ := json.Marshal(d)
	return x
}

// compactJson will remove insignificant spaces
func compactJson(jsonStr string) (string, error) {

	dst := new(bytes.Buffer)
	src := []byte(jsonStr)

	err := json.Compact(dst, src)
	if err != nil {
		return "", err
	}

	return dst.String(), nil
}

func recaptchaCheck(recaptchaCode string) error {

	if recaptchaSecret == "" {
		return nil
	}

	resp, err := http.PostForm("https://www.google.com/recaptcha/api/siteverify", url.Values{
		"secret":   {recaptchaSecret},
		"response": {recaptchaCode}})
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	type Root struct {
		Success bool          `json:"success"`
		Errors  []interface{} `json:"error-codes"`
	}

	var root Root
	err = json.Unmarshal(body, &root)
	if err != nil {
		return err
	}

	if root.Success {
		return nil
	}

	log.Println(fmt.Sprintf("recaptcha error: %v", root.Errors))
	return errors.New("recaptcha failed")
}
