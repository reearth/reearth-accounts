package http_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/adapter"
	adapterhttp "github.com/reearth/reearth-accounts/server/internal/adapter/http"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	gatewaymock "github.com/reearth/reearth-accounts/server/internal/usecase/gateway/mock"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/appx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// stubAuthResolver always returns unauthenticated (no user resolved).
func stubAuthResolver(_ echo.Context, _ *appx.AuthInfo) (*user.User, *workspace.Operator, error) {
	return nil, nil, nil
}

type stubAuthConfigProvider struct{}

func (stubAuthConfigProvider) GetAuth0Domain() string      { return "" }
func (stubAuthConfigProvider) GetAuth0Audience() string    { return "" }
func (stubAuthConfigProvider) GetAuth0WebClientID() string { return "" }
func (stubAuthConfigProvider) GetAuthProvider() string     { return "auth0" }
func (stubAuthConfigProvider) GetCIPAPIKey() string        { return "" }
func (stubAuthConfigProvider) GetCIPAuthDomain() string    { return "" }
func (stubAuthConfigProvider) GetCIPProjectID() string     { return "" }
func (stubAuthConfigProvider) GetCIPTenantID() string      { return "" }

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

// noSigning is a signer for a test that must not sign a photo URL: any call
// fails the test.
func noSigning(t *testing.T) adapter.PhotoSigner {
	return photoSigner(t, gatewaymock.NewMockStorage(gomock.NewController(t)))
}

func photoSigner(t *testing.T, storage gateway.Storage) adapter.PhotoSigner {
	t.Helper()
	p, err := adapter.NewPhotoSigner(storage)
	require.NoError(t, err)
	return p
}

func TestRegisterRESTRouter_CacheControlHeader(t *testing.T) {
	e := newRESTEcho(adapterhttp.RouterConfig{
		AuthConfigProvider: stubAuthConfigProvider{},
		Photos:             noSigning(t),
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
		Photos:             noSigning(t),
		// CacheControl deliberately omitted
	})

	req := httptest.NewRequest(http.MethodGet, "/api/auth/config", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("Cache-Control"))
}

// TestServiceRoutes_AuthGate verifies that the routes the static M2M keys used to
// guard admit a user or a service caller and reject a bearer the resolver does
// not accept, such as a former static key (ADR-0002 decision 7.1).
func TestServiceRoutes_AuthGate(t *testing.T) {
	iid := id.NewIntegrationID()
	resolver := func(c echo.Context, _ *appx.AuthInfo) (*user.User, *workspace.Operator, error) {
		if c.Request().Header.Get(echo.HeaderAuthorization) == "Bearer service-token" {
			return nil, &workspace.Operator{Integration: &iid}, nil
		}
		return nil, nil, nil
	}
	e := newRESTEcho(adapterhttp.RouterConfig{
		AuthConfigProvider: stubAuthConfigProvider{},
		Photos:             noSigning(t),
		AuthResolver:       resolver,
	})

	do := func(path, authHeader string) int {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		if authHeader != "" {
			req.Header.Set(echo.HeaderAuthorization, authHeader)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec.Code
	}

	for _, path := range []string{"/api/users/sync-sso", "/api/users/find-or-create", "/api/permissions/check"} {
		assert.Equal(t, http.StatusUnauthorized, do(path, ""), "%s: no credentials", path)
		assert.Equal(t, http.StatusUnauthorized, do(path, "Bearer former-static-key"), "%s: a static key is no longer accepted", path)
	}
	for _, path := range []string{"/api/users/sync-sso", "/api/users/find-or-create"} {
		assert.NotEqual(t, http.StatusUnauthorized, do(path, "Bearer service-token"), "%s: a service caller passes the gate", path)
	}
}
