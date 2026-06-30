//go:build wireinject
// +build wireinject

package di

import (
	"github.com/goforj/wire"
)

// gatewayWire provides external-service clients (Cerbos).
var gatewayWire = wire.NewSet(
	provideCerbosClient,
)