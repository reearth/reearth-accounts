package main

import (
	"github.com/reearth/reearth-accounts/server/internal/reearth-accounts-admin/di"
	"github.com/reearth/reearthx/log"
)

// @title						Re:Earth Accounts Admin API
// @version					1.0
// @description				Admin REST API for the internal management console.
// @BasePath					/api/v1
// @securityDefinitions.apikey	BearerAuth
// @in							header
// @name						Authorization
func main() {
	server, cleanup, err := di.InitializeEcho()
	if err != nil {
		log.Fatalf("failed to initialize admin api: %v", err)
	}
	defer cleanup()

	server.Start()
}
