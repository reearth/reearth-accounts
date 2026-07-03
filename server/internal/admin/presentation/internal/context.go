// Package internal holds presentation-layer helpers (auth context access and
// the shared error response shape) for the admin API.
package internal

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
	"github.com/reearth/reearth-accounts/server/pkg/user"
)

const userContextKey = "admin:auth:user"

const sessionAdminUserIDKey = "admin:session:adminuserid"

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
	return nil, echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
}

// SetSessionAdminUserID stores the admin user ID parsed from the session token
// in the echo context. It must only be called from the session middleware.
func SetSessionAdminUserID(c echo.Context, id adminuser.ID) {
	c.Set(sessionAdminUserIDKey, id)
}

// GetSessionAdminUserID retrieves the session admin user ID, returning 401 if
// absent. It must only be called from handlers behind the session middleware.
func GetSessionAdminUserID(c echo.Context) (adminuser.ID, error) {
	if id, ok := c.Get(sessionAdminUserIDKey).(adminuser.ID); ok && !id.IsEmpty() {
		return id, nil
	}
	return adminuser.ID{}, echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
}
