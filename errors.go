// Copyright 2019
// The Honest Scoop and P.J. Siripala
// All rights reserved

package main

import (
	"fmt"
)

func ErrorFmt(msg interface{}) map[string]interface{} {
	return map[string]interface{}{
		"error": fmt.Sprintf("%v", msg),
	}
}
