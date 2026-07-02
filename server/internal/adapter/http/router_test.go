package http_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	adapterhttp "github.com/reearth/reearth-accounts/server/internal/adapter/http"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/appx"
	"github.com/stretchr/testify/assert"
)

// stubAuthResolver always returns unauthenticated (no user resolved).
func stubAuthResolver(_ echo.Context, _ *appx.AuthInfo) (*user.User, *workspace.Operator, error) {
	return nil, nil, nil
}

type stubAuthConfigProvider struct{}

func (stubAuthConfigProvider) GetAuth0Domain() string    { return "" }
func (stubAuthConfigProvider) GetAuth0Audience() string  { return "" }
func (stubAuthConfigProvider) GetAuth0WebClientID() string { return "" }
func (stubAuthConfigProvider) GetAuthProvider() string   { return "auth0" }
func (stubAuthConfigProvider) GetCIPAPIKey() string      { return "" }
func (stubAuthConfigProvider) GetCIPAuthDomain() string  { return "" }
func (stubAuthConfigProvider) GetCIPProjectID() string   { return "" }
func (stubAuthConfigProvider) GetCIPTenantID() string    { return "" }

func cacheControlMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Header().Set("Cache-Control", "private")
		return next(c)
	}
}

func newRESTEcho(cfg adapterhttp.RouterConfig) *echo.Echo {
	e := echo.New()
	adapterhttp.RegisterRESTRouter(e, cfg)
	return e
}

func TestRegisterRESTRouter_CacheControlHeader(t *testing.T) {
	e := newRESTEcho(adapterhttp.RouterConfig{
		AuthConfigProvider: stubAuthConfigProvider{},
		CacheControl:       cacheControlMiddleware,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/auth/config", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "private", rec.Header().Get("Cache-Control"))
}

func TestRegisterRESTRouter_NoCacheControlWithoutMiddleware(t *testing.T) {
	e := newRESTEcho(adapterhttp.RouterConfig{
		AuthConfigProvider: stubAuthConfigProvider{},
		// CacheControl deliberately omitted
	})

	req := httptest.NewRequest(http.MethodGet, "/api/auth/config", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("Cache-Control"))
}

// TestSyncSSORoute_AuthGate verifies that the sync-sso route is protected by
// SyncSSOAPIKey and is not accessible with the general RestAPIKey or no credentials.
func TestSyncSSORoute_AuthGate(t *testing.T) {
	const restKey = "rest-key"
	const syncKey = "sync-key"

	e := newRESTEcho(adapterhttp.RouterConfig{
		AuthConfigProvider: stubAuthConfigProvider{},
		AuthResolver:       stubAuthResolver,
		APIKey:             restKey,
		SyncSSOAPIKey:      syncKey,
	})

	doPost := func(authHeader string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/users/sync-sso", nil)
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec.Code
	}

	assert.Equal(t, http.StatusUnauthorized, doPost(""), "no credentials → 401")
	assert.Equal(t, http.StatusUnauthorized, doPost("Bearer "+restKey), "RestAPIKey must not grant access to sync-sso")
	assert.NotEqual(t, http.StatusUnauthorized, doPost("Bearer "+syncKey), "SyncSSOAPIKey must pass the auth gate")
}

// TestSyncSSORoute_KeyIsolation verifies that the SyncSSOAPIKey is not accepted on
// routes guarded by the general RestAPIKey (find-or-create, permissions/check).
func TestSyncSSORoute_KeyIsolation(t *testing.T) {
	const syncKey = "sync-key"

	e := newRESTEcho(adapterhttp.RouterConfig{
		AuthConfigProvider: stubAuthConfigProvider{},
		AuthResolver:       stubAuthResolver,
		SyncSSOAPIKey:      syncKey,
		// APIKey (RestAPIKey) deliberately omitted
	})

	routes := []string{
		"/api/users/find-or-create",
		"/api/permissions/check",
	}
	for _, path := range routes {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.Header.Set("Authorization", "Bearer "+syncKey)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "SyncSSOAPIKey must not grant access to %s", path)
	}
}
