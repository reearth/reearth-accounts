package http_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	adapterhttp "github.com/reearth/reearth-accounts/server/internal/adapter/http"
	"github.com/stretchr/testify/assert"
)

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
