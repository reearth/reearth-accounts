package http

import (
	"crypto/subtle"
	"strings"

	"github.com/labstack/echo/v4"
	httpinternal "github.com/reearth/reearth-accounts/server/internal/adapter/http/internal"
)

const bearerPrefix = "Bearer "

// skipJWTOnAPIKey wraps the JWT middleware so a request whose bearer token equals
// any of the configured M2M API keys bypasses JWT validation and reaches the
// per-route key middleware. Without this, the JWT validator (which runs first on
// the /api group) would reject the API key as a malformed JWT before the
// API-key middleware ever runs, effectively breaking the advertised M2M flow.
func skipJWTOnAPIKey(jwt echo.MiddlewareFunc, apiKeys ...string) echo.MiddlewareFunc {
	if jwt == nil {
		return jwt
	}
	keys := make([]string, 0, len(apiKeys))
	for _, k := range apiKeys {
		if k != "" {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return jwt
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			h := c.Request().Header.Get(echo.HeaderAuthorization)
			if strings.HasPrefix(h, bearerPrefix) {
				token := strings.TrimPrefix(h, bearerPrefix)
				for _, k := range keys {
					if subtle.ConstantTimeCompare([]byte(token), []byte(k)) == 1 {
						return next(c)
					}
				}
			}
			return jwt(next)(c)
		}
	}
}

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
