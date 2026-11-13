package main

import (
	"context"
	"github.com/reearth/reearth-accounts/server/internal/admin/di"
	"github.com/reearth/reearth-accounts/server/pkg/applog"
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
		applog.Fatal(context.Background(), "failed to initialize admin api", applog.Err(err))
	}
	defer cleanup()

	server.Start()
}
