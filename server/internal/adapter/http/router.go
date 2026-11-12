package http

import (
	"crypto/subtle"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	_ "github.com/reearth/reearth-accounts/server/docs" // generated OpenAPI spec (make swag)
	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/internal/adapter/http/handlers"
	httpinternal "github.com/reearth/reearth-accounts/server/internal/adapter/http/internal"
	echoSwagger "github.com/swaggo/echo-swagger"
)

// RouterConfig carries dependencies for REST registration, supplied by the app layer.
type RouterConfig struct {
	// AuthResolver reuses app/auth.go's FindBySub + operator builder pipeline.
	AuthResolver AuthResolver
	// CacheControl sets response cache headers (e.g. Cache-Control: private) on all REST responses.
	CacheControl echo.MiddlewareFunc
	// UsecaseMiddleware attaches *interfaces.Container to context (same as GraphQL path).
	UsecaseMiddleware echo.MiddlewareFunc
	// JWTMiddleware validates the bearer token into adapter.AuthInfoKey (appx). May be nil under mock auth.
	JWTMiddleware echo.MiddlewareFunc
	// AuthConfigProvider exposes Auth0 settings for GET /api/auth/config.
	AuthConfigProvider adapter.Auth0ConfigProvider
	// Photos signs photo URLs.
	Photos adapter.PhotoSigner
	// Swagger basic-auth (optional).
	SwaggerUser, SwaggerPass string
	// LoginTime reads the login time for the recovery code step-up check.
	LoginTime adapter.LoginTimeSource
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
		base = append(base, cfg.JWTMiddleware)
	}
	if cfg.UsecaseMiddleware != nil {
		base = append(base, cfg.UsecaseMiddleware)
	}
	if cfg.CacheControl != nil {
		base = append(base, cfg.CacheControl)
	}

	required := RequiredAuth(cfg.AuthResolver)
	userOnly := RequiredUser(cfg.AuthResolver)
	optional := OptionalAuth(cfg.AuthResolver)

	api := e.Group("/api", base...)

	// --- Auth ---
	ah := handlers.NewAuthHandler(cfg.AuthConfigProvider)
	api.GET("/auth/config", ah.Config)            // public
	api.POST("/auth/logout", ah.Logout, userOnly) // JWT

	// --- Users ---
	uh := handlers.NewUserHandler(cfg.Photos, cfg.LoginTime)
	api.GET("/users/me", uh.Me, userOnly, handlers.Expand(handlers.ExpandMyWorkspace))
	api.PATCH("/users/me", uh.UpdateMe, userOnly)
	api.DELETE("/users/me", uh.DeleteMe, userOnly)
	api.DELETE("/users/me/auths/:sub", uh.RemoveMyAuth, userOnly)
	api.GET("/users/me/mfa", uh.GetMFAStatus, userOnly)
	api.POST("/users/me/mfa", uh.EnableMFA, userOnly)
	api.DELETE("/users/me/mfa", uh.DisableMFA, userOnly)
	api.POST("/users/me/mfa/recovery-code", uh.RegenerateMFARecoveryCode, userOnly)
	api.GET("/users/search", uh.Search, userOnly)
	api.GET("/users/by-alias", uh.FindByAlias, userOnly)
	api.GET("/users/by-name-or-email", uh.FindByNameOrEmail, userOnly)
	api.GET("/users/by-name-or-alias", uh.FindByNameOrAlias, userOnly)
	api.GET("/users", uh.List, userOnly) // ?ids= | ?ids=&alias=&page=&page_size=
	// Cross-tenant listing for service-to-service callers (e.g. LINKS-Veda). The
	// interactor admits only maintainers, and the response includes every email.
	api.GET("/users/all", uh.ListAll, userOnly) // ?keyword=&status=active|deleted|all&page=&page_size=
	api.GET("/users/:id", uh.Get, userOnly)
	api.POST("/users/:id/deactivate", uh.Deactivate, userOnly) // soft delete (maintainer-only)
	api.POST("/users/:id/restore", uh.Restore, userOnly)       // undo deactivate (maintainer-only)
	api.POST("/users/signup", uh.Signup, optional)
	api.POST("/users/signup-oidc", uh.SignupOIDC, optional)
	// Called by the identity provider's post-login hook as a service caller.
	api.POST("/users/sync-sso", uh.SyncSSOUser, required)
	api.POST("/users/verifications", uh.CreateVerification, optional)
	api.POST("/users/verify", uh.VerifyUser)
	api.POST("/users/password-reset/start", uh.StartPasswordReset)
	api.POST("/users/password-reset", uh.PasswordReset)
	api.POST("/users/find-or-create", uh.FindOrCreate, required)
	// PATCH /api/users/by-sub/:sub — JWT required; caller must hold the maintainer
	// role (Cerbos, falling back to a direct Permittable check). Updates mutable
	// user fields (name) by Firebase sub.
	api.PATCH("/users/by-sub/:sub", uh.UpdateUserBySub, userOnly)
	// PUT /api/users/by-sub/:sub/platform-roles — JWT required; same maintainer-role
	// gate as above. Replaces platform roles by Firebase sub.
	api.PUT("/users/by-sub/:sub/platform-roles", uh.SetPlatformRolesBySub, userOnly)

	// --- Workspaces ---
	wh := handlers.NewWorkspaceHandler(cfg.Photos)
	wsExpand := handlers.Expand(handlers.ExpandMemberUsers)
	api.POST("/workspaces", wh.Create, required, wsExpand)
	api.GET("/workspaces", wh.List, required, wsExpand) // ?ids= | ?name= | ?alias= | ?user_id=(&page=&page_size=)
	// Cross-tenant listing for service-to-service callers (e.g. LINKS-Veda). The
	// interactor admits only maintainers.
	api.GET("/workspaces/all", wh.ListAll, required, wsExpand) // ?keyword=&status=active|deleted|all&page=&page_size=
	api.GET("/workspaces/:id", wh.Get, required, wsExpand)
	api.PATCH("/workspaces/:id", wh.Update, required, wsExpand)
	api.DELETE("/workspaces/:id", wh.Delete, required)
	api.POST("/workspaces/:id/deactivate", wh.Deactivate, required, wsExpand) // soft delete (owner-only)
	api.POST("/workspaces/:id/restore", wh.Restore, required, wsExpand)       // undo deactivate (owner-only)
	api.POST("/workspaces/:id/members", wh.AddMembers, required, wsExpand)
	api.PATCH("/workspaces/:id/members/:user_id", wh.UpdateMember, required, wsExpand)
	api.DELETE("/workspaces/:id/members/:user_id", wh.RemoveMember, required, wsExpand)
	api.DELETE("/workspaces/:id/members", wh.RemoveMembers, required, wsExpand)
	api.POST("/workspaces/:id/integrations", wh.AddIntegration, required, wsExpand)
	api.PATCH("/workspaces/:id/integrations/:integration_id", wh.UpdateIntegration, required, wsExpand)
	api.DELETE("/workspaces/:id/integrations/:integration_id", wh.RemoveIntegration, required, wsExpand)
	api.DELETE("/workspaces/:id/integrations", wh.RemoveIntegrations, required, wsExpand)
	api.POST("/workspaces/:id/transfer-ownership", wh.TransferOwnership, required, wsExpand)

	// --- Workspace SCIM admin ---
	sh := handlers.NewWorkspaceScimHandler()
	api.GET("/workspaces/:id/scim", sh.GetState, required)
	api.PATCH("/workspaces/:id/scim", sh.SetAvailable, required)
	api.POST("/workspaces/:id/scim/token", sh.GenerateToken, required)
	api.DELETE("/workspaces/:id/scim/token", sh.RevokeToken, required)
	api.PUT("/workspaces/:id/scim/config", sh.UpdateConfig, required)
	api.GET("/workspaces/:id/scim/config", sh.GetConfig, required)

	// --- Service routes ---
	// JWT required; the caller must hold Maintainer or Owner in the target workspace
	// This can bypass the self-promotion guard that PATCH .../members/:user_id enforces.
	api.PATCH("/service/workspaces/:id/members/:user_id", wh.UpdateMemberViaService, required, wsExpand)

	// --- Permission ---
	ph := handlers.NewPermissionHandler()
	api.POST("/permissions/check", ph.Check, required)

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
