//go:build wireinject
// +build wireinject

package di

import (
	"github.com/goforj/wire"
	"github.com/reearth/reearth-accounts/server/internal/admin/presentation"
	adminuserhandler "github.com/reearth/reearth-accounts/server/internal/admin/presentation/handler/adminuser"
	authhandler "github.com/reearth/reearth-accounts/server/internal/admin/presentation/handler/auth"
	userhandler "github.com/reearth/reearth-accounts/server/internal/admin/presentation/handler/user"
	workspacehandler "github.com/reearth/reearth-accounts/server/internal/admin/presentation/handler/workspace"
)

// handlerWire provides the per-resource handlers and the aggregated Handler.
var handlerWire = wire.NewSet(
	adminuserhandler.NewHandler,
	authhandler.NewHandler,
	provideCookieSecure,
	userhandler.NewHandler,
	workspacehandler.NewHandler,
	presentation.NewHandler,
)
