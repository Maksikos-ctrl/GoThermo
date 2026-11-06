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
//	DATABASE_URL    Postgres, needed only for the migrate command
//
// One-off move of the data from Redis to Postgres (see redis_to_postgres.go):
//
//	gothermo-server migrate -dry-run
//	gothermo-server migrate
//
// Encrypt the text of messages written before GOTHERMO_DATA_KEY was set (or
// undo it with -decrypt), see crypt.go:
//
//	gothermo-server encrypt -dry-run
//	gothermo-server encrypt
func main() {
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		initRedis()
		os.Exit(runMigrateCommand(os.Args[2:], os.Stdin, os.Stdout))
	}
	if len(os.Args) > 1 && os.Args[1] == "encrypt" {
		os.Exit(runEncryptCommand(os.Args[2:], os.Stdout))
	}
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
