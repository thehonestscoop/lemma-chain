package main

import (
	"fmt"
)

func ErrorFmt(msg interface{}) map[string]interface{} {
	return map[string]interface{}{
		"error": fmt.Sprintf("%v", msg),
	}
}
