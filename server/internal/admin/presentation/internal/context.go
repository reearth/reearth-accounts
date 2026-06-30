// Package internal holds presentation-layer helpers (auth context access and
// the shared error response shape) for the admin API.
package internal

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/pkg/user"
)

const userContextKey = "admin:auth:user"

// SetUser stores the authenticated admin user in the echo context.
// It must only be called from the auth middleware.
func SetUser(c echo.Context, u *user.User) {
	c.Set(userContextKey, u)
}

// GetUser retrieves the authenticated admin user, returning 401 if absent.
// It must only be called from handlers.
func GetUser(c echo.Context) (*user.User, error) {
	if u, ok := c.Get(userContextKey).(*user.User); ok && u != nil {
		return u, nil
	}
	return nil, echo.NewHTTPError(http.StatusUnauthorized, "user not found in context")
}
