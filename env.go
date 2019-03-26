// Copyright 2019
// The Honest Scoop and P.J. Siripala
// All rights reserved

package main

import (
	"log"
	"os"
	"strconv"
)

func lookupEnvOrUseDefault(key string, defaultValue string) string {
	val, found := os.LookupEnv(key)
	if found {
		return val
	} else {
		return defaultValue
	}
}

func lookupEnvOrUseDefaultInt(key string, defaultValue int) int {
	val, found := os.LookupEnv(key)
	if found {
		i, err := strconv.Atoi(val)
		if err != nil {
			log.Fatal(err)
		}
		return i
	} else {
		return defaultValue
	}
}

func lookupEnvOrUseDefaultInt64(key string, defaultValue int64) int64 {
	val, found := os.LookupEnv(key)
	if found {
		i, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			log.Fatal(err)
		}
		return i
	} else {
		return defaultValue
	}
}

func lookupEnvOrUseDefaultFloat64(key string, defaultValue float64) float64 {
	val, found := os.LookupEnv(key)
	if found {
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			log.Fatal(err)
		}
		return f
	} else {
		return defaultValue
	}
}
