package http

import (
	"crypto/subtle"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/internal/adapter/http/handlers"
	httpinternal "github.com/reearth/reearth-accounts/server/internal/adapter/http/internal"
	_ "github.com/reearth/reearth-accounts/server/docs" // generated OpenAPI spec (make swag)
	echoSwagger "github.com/swaggo/echo-swagger"
)

// RouterConfig carries dependencies for REST registration, supplied by the app layer.
type RouterConfig struct {
	// AuthResolver reuses app/auth.go's FindBySub + operator builder pipeline.
	AuthResolver AuthResolver
	// UsecaseMiddleware attaches *interfaces.Container to context (same as GraphQL path).
	UsecaseMiddleware echo.MiddlewareFunc
	// JWTMiddleware validates the bearer token into adapter.AuthInfoKey (appx). May be nil under mock auth.
	JWTMiddleware echo.MiddlewareFunc
	// AuthConfigProvider exposes Auth0 settings for GET /api/auth/config.
	AuthConfigProvider adapter.Auth0ConfigProvider
	// APIKey is the M2M key for service-to-service routes.
	APIKey string
	// Swagger basic-auth (optional).
	SwaggerUser, SwaggerPass string
	// Debug enables serving /swagger without credentials (debug/dev only); in
	// production Swagger is served only when basic-auth credentials are configured.
	Debug bool
}

// RegisterRESTRouter mounts all /api/* REST routes additively. It MUST be called after
// the GraphQL route is registered; it shares the same Echo instance and global middleware.
func RegisterRESTRouter(e *echo.Echo, cfg RouterConfig) {
	// Errors from REST handlers render as structured ErrorResponse JSON.
	// Wrapped so the existing GraphQL HTTPErrorHandler is preserved for non-REST paths.
	prev := e.HTTPErrorHandler
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		if pathHasPrefix(c, "/api/") && !pathHasPrefix(c, "/api/graphql") {
			httpinternal.CustomHTTPErrorHandler(err, c)
			return
		}
		prev(err, c)
	}

	base := []echo.MiddlewareFunc{}
	if cfg.JWTMiddleware != nil {
		base = append(base, cfg.JWTMiddleware) // populates adapter.AuthInfoKey, non-fatal
	}
	if cfg.UsecaseMiddleware != nil {
		base = append(base, cfg.UsecaseMiddleware)
	}

	required := RequiredAuth(cfg.AuthResolver)
	optional := OptionalAuth(cfg.AuthResolver)
	apikeyOrAuth := APIKeyOrAuth(cfg.APIKey)

	api := e.Group("/api", base...)

	// --- Auth ---
	ah := handlers.NewAuthHandler(cfg.AuthConfigProvider)
	api.GET("/auth/config", ah.Config)            // public
	api.POST("/auth/logout", ah.Logout, required) // JWT

	// --- Users ---
	uh := handlers.NewUserHandler()
	api.GET("/users/me", uh.Me, required)
	api.PATCH("/users/me", uh.UpdateMe, required)
	api.DELETE("/users/me", uh.DeleteMe, required)
	api.DELETE("/users/me/auths/:sub", uh.RemoveMyAuth, required)
	api.GET("/users/search", uh.Search, required)
	api.GET("/users/by-alias", uh.FindByAlias, required)
	api.GET("/users/by-name-or-email", uh.FindByNameOrEmail, required)
	api.GET("/users/by-name-or-alias", uh.FindByNameOrAlias, required)
	api.GET("/users", uh.List, required) // ?ids= | ?ids=&alias=&page=&page_size=
	api.GET("/users/:id", uh.Get, required)
	api.POST("/users/signup", uh.Signup, optional)
	api.POST("/users/signup-oidc", uh.SignupOIDC, optional)
	api.POST("/users/verifications", uh.CreateVerification, optional)
	api.POST("/users/verify", uh.VerifyUser)
	api.POST("/users/password-reset/start", uh.StartPasswordReset)
	api.POST("/users/password-reset", uh.PasswordReset)
	api.POST("/users/find-or-create", uh.FindOrCreate, optional, apikeyOrAuth)

	// --- Workspaces ---
	wh := handlers.NewWorkspaceHandler()
	api.POST("/workspaces", wh.Create, required)
	api.GET("/workspaces", wh.List, required) // ?ids= | ?name= | ?alias= | ?user_id=(&page=&page_size=)
	api.GET("/workspaces/:id", wh.Get, required)
	api.PATCH("/workspaces/:id", wh.Update, required)
	api.DELETE("/workspaces/:id", wh.Delete, required)
	api.POST("/workspaces/:id/members", wh.AddMembers, required)
	api.PATCH("/workspaces/:id/members/:user_id", wh.UpdateMember, required)
	api.DELETE("/workspaces/:id/members/:user_id", wh.RemoveMember, required)
	api.DELETE("/workspaces/:id/members", wh.RemoveMembers, required)
	api.POST("/workspaces/:id/integrations", wh.AddIntegration, required)
	api.PATCH("/workspaces/:id/integrations/:integration_id", wh.UpdateIntegration, required)
	api.DELETE("/workspaces/:id/integrations/:integration_id", wh.RemoveIntegration, required)
	api.DELETE("/workspaces/:id/integrations", wh.RemoveIntegrations, required)
	api.POST("/workspaces/:id/transfer-ownership", wh.TransferOwnership, required)

	// --- Permission ---
	ph := handlers.NewPermissionHandler()
	// OptionalAuth resolves a JWT/mock user (attaching it for APIKeyOrAuth and the
	// handler's RequireUser); APIKeyOrAuth then admits either that user or a valid M2M key.
	api.POST("/permissions/check", ph.Check, optional, apikeyOrAuth)

	// --- Swagger ---
	// Basic-auth protected when credentials are configured (served in any environment);
	// otherwise served only in debug/dev, mirroring how the GraphQL playground is gated,
	// so API docs aren't unintentionally exposed in production.
	// BOTH user and password must be set to enable basic-auth Swagger; if only one is
	// configured (e.g. password env var accidentally omitted) we refuse to mount Swagger
	// rather than allowing an empty password to gate it.
	switch {
	case cfg.SwaggerUser != "" && cfg.SwaggerPass != "":
		e.GET("/swagger/*", echoSwagger.WrapHandler, middleware.BasicAuth(func(u, p string, _ echo.Context) (bool, error) {
			// constant-time comparison to avoid leaking credential length/match via timing
			userOK := subtle.ConstantTimeCompare([]byte(u), []byte(cfg.SwaggerUser)) == 1
			passOK := subtle.ConstantTimeCompare([]byte(p), []byte(cfg.SwaggerPass)) == 1
			return userOK && passOK, nil
		}))
	case cfg.Debug:
		e.GET("/swagger/*", echoSwagger.WrapHandler)
	}
}

func pathHasPrefix(c echo.Context, prefix string) bool {
	p := c.Request().URL.Path
	return len(p) >= len(prefix) && p[:len(prefix)] == prefix
}
