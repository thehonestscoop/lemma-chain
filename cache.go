// Copyright 2019
// The Honest Scoop and P.J. Siripala
// All rights reserved

package main

import (
	"time"

	"github.com/patrickmn/go-cache"
)

var memoryCache cacher

type cacher interface {
	Get(k string) (interface{}, bool)
	Set(k string, x interface{}, d time.Duration)
}

// noCache is used to disable caching
type noCache struct{}

func (nc *noCache) Get(k string) (interface{}, bool) {
	return nil, false
}

func (nc *noCache) Set(k string, x interface{}, d time.Duration) {
	return
}

func init() {
	if cacheDuration != 0 {
		memoryCache = cache.New(time.Duration(cacheDuration)*time.Minute, 10*time.Minute)
	} else {
		// Cache is disabled
		memoryCache = &noCache{}
	}
}
