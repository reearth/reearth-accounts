//go:build wireinject
// +build wireinject

package di

import (
	"github.com/goforj/wire"
)

// InitializeEcho builds the fully-wired admin Echo server. The returned cleanup
// function tears down resources (e.g. the DB connection).
func InitializeEcho() (*Server, func(), error) {
	wire.Build(
		// config
		LoadConfig,

		// repositories
		repoWire,

		// gateways
		gatewayWire,

		// usecases
		usecaseWire,

		// handlers
		handlerWire,

		// middleware
		middlewareWire,

		// echo server
		NewAppEchoServer,
	)
	return nil, nil, nil
}
