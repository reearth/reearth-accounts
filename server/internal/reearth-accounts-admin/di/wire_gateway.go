package di

import (
	"github.com/goforj/wire"
)

// gatewayWire provides external-service gateways (Cerbos).
var gatewayWire = wire.NewSet(
	provideCerbosGateway,
)
