//go:build server

package main

import (
	"context"
	"log"
	"os"
)

// Headless entry point: go build -tags server -o gothermo-server .
// Settings come from environment variables:
//
//	GOTHERMO_ADDR   listen address, default 0.0.0.0:8081
//	REDIS_ADDR      default localhost:6379
//	REDIS_PASSWORD  default empty
func main() {
	addr := os.Getenv("GOTHERMO_ADDR")
	if addr == "" {
		addr = "0.0.0.0:8081"
	}
	app := NewApp()
	app.startup(context.Background())
	if err := StartServer(app, app.hub, addr); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
