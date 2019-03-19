package main

import (
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
	limiter := tollbooth.NewLimiter(1, nil)
	limiter.SetIPLookups([]string{"RemoteAddr", "X-Forwarded-For", "X-Real-IP"})

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(tollbooth_echo.LimitHandler(limiter))
	e.Use(nocache)
	e.Use(loginChecker)

	// Routes
	e.POST("/accounts", createAccountHandler)
	e.GET("/accounts/:name", showAccount)
	e.POST("/ref", createNodeHandler)
	e.GET("*", findChainHandler)

	// Start server
	e.Logger.Fatal(e.Start(":1323"))
}
