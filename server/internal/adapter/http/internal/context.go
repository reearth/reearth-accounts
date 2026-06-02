package internal

import (
	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
)

// Operator returns the workspace operator attached by the auth middleware, or nil.
func Operator(c echo.Context) *workspace.Operator {
	return adapter.Operator(c.Request().Context())
}

// User returns the authenticated user attached by the auth middleware, or nil.
func User(c echo.Context) *user.User {
	return adapter.User(c.Request().Context())
}

// Usecases returns the interactor container attached by the usecase middleware.
func Usecases(c echo.Context) *interfaces.Container {
	return adapter.Usecases(c.Request().Context())
}

// RequireUser returns the authenticated user or ErrUnauthorized.
func RequireUser(c echo.Context) (*user.User, error) {
	u := User(c)
	if u == nil {
		return nil, ErrUnauthorized
	}
	return u, nil
}
