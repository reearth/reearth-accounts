//go:build wireinject
// +build wireinject

package di

import (
	"github.com/goforj/wire"
	"github.com/reearth/reearth-accounts/server/internal/admin/presentation"
	mw "github.com/reearth/reearth-accounts/server/internal/admin/presentation/middleware"
)

// middlewareWire provides the session/approval middlewares and the application
// middleware bundle.
var middlewareWire = wire.NewSet(
	mw.NewSessionMiddleware,
	mw.NewRequireApprovedMiddleware,
	presentation.NewAppMiddlewares,
)
