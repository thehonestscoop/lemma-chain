// Copyright 2019
// The Honest Scoop and P.J. Siripala
// All rights reserved

package main

import (
	"fmt"
	"log"

	"github.com/dgraph-io/dgo"
	"github.com/dgraph-io/dgo/protos/api"
	"github.com/didip/tollbooth"
	"github.com/didip/tollbooth_echo"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"google.golang.org/grpc"
)

var dg *dgo.Dgraph

func init() {

	conn, err := grpc.Dial("127.0.0.1:9080", grpc.WithInsecure())
	if err != nil {
		log.Fatal("While trying to dial gRPC")
	}

	dg = dgo.NewDgraphClient(api.NewDgraphClient(conn))

	setSchema()
}

func main() {

	// Echo instance
	e := echo.New()

	// Middleware
	limiter := tollbooth.NewLimiter(rateLimit, nil)
	if behindProxy == 1 {
		limiter.SetIPLookups([]string{"X-Forwarded-For", "RemoteAddr", "X-Real-IP"})
	} else {
		limiter.SetIPLookups([]string{"RemoteAddr", "X-Forwarded-For", "X-Real-IP"})
	}

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())
	e.Use(tollbooth_echo.LimitHandler(limiter))
	e.Use(nocache)
	e.Use(loginChecker)

	// Routes
	e.POST("/accounts", createAccountHandler)
	e.GET("/accounts/:name", showAccountHandler)
	e.POST("/ref", createNodeHandler)
	e.GET("/verify/:code", verifyHandler)
	e.GET("/search/:terms", searchHandler) // Cached
	e.GET("*", findChainHandler)           // Cached

	// Start server
	e.Logger.Fatal(e.Start(fmt.Sprintf(":%d", listenPort)))
}
