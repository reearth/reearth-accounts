//go:build wireinject
// +build wireinject

package di

import (
	"github.com/goforj/wire"
	"github.com/reearth/reearth-accounts/server/internal/reearth-accounts-admin/presentation"
	userhandler "github.com/reearth/reearth-accounts/server/internal/reearth-accounts-admin/presentation/handler/user"
)

// handlerWire provides the per-resource handlers and the aggregated Handler.
var handlerWire = wire.NewSet(
	userhandler.NewHandler,
	presentation.NewHandler,
)