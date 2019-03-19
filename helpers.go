package main

import (
	"bytes"
	"encoding/json"
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
