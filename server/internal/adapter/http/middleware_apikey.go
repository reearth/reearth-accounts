package http

import (
	"crypto/subtle"
	"strings"

	"github.com/labstack/echo/v4"
	httpinternal "github.com/reearth/reearth-accounts/server/internal/adapter/http/internal"
)

const bearerPrefix = "Bearer "

// APIKeyOrAuth allows a request through if EITHER a valid M2M API key is presented
// (Authorization: Bearer <key> matching cfgKey) OR the user is already authenticated
// by a preceding JWT middleware. Used for service-to-service routes (findOrCreate,
// checkPermission) that accept JWT or M2M.
func APIKeyOrAuth(cfgKey string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if httpinternal.User(c) != nil {
				return next(c)
			}
			if cfgKey != "" {
				h := c.Request().Header.Get(echo.HeaderAuthorization)
				if strings.HasPrefix(h, bearerPrefix) {
					token := strings.TrimPrefix(h, bearerPrefix)
					if subtle.ConstantTimeCompare([]byte(token), []byte(cfgKey)) == 1 {
						return next(c)
					}
				}
			}
			return httpinternal.ErrUnauthorized
		}
	}
}
