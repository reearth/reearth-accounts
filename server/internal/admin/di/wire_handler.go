//go:build wireinject
// +build wireinject

package di

import (
	"github.com/goforj/wire"
	"github.com/reearth/reearth-accounts/server/internal/admin/presentation"
	authhandler "github.com/reearth/reearth-accounts/server/internal/admin/presentation/handler/auth"
	userhandler "github.com/reearth/reearth-accounts/server/internal/admin/presentation/handler/user"
)

// handlerWire provides the per-resource handlers and the aggregated Handler.
var handlerWire = wire.NewSet(
	authhandler.NewHandler,
	provideCookieSecure,
	userhandler.NewHandler,
	presentation.NewHandler,
)