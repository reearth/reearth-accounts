package di

import (
	"github.com/goforj/wire"
	"github.com/reearth/reearth-accounts/server/internal/reearth-accounts-admin/presentation"
	mw "github.com/reearth/reearth-accounts/server/internal/reearth-accounts-admin/presentation/middleware"
)

// middlewareWire provides the JWT providers, the auth middleware, and the
// application middleware bundle.
var middlewareWire = wire.NewSet(
	provideJWTProviders,
	mw.NewAuthMiddleware,
	presentation.NewAppMiddlewares,
)
