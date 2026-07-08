package presentation

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestCacheControlMiddleware(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := cacheControlMiddleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	assert.NoError(t, handler(c))
	assert.Equal(t, "private, no-store", rec.Header().Get("Cache-Control"))
}

func TestNewAppMiddlewares_CacheControlSet(t *testing.T) {
	m := NewAppMiddlewares()
	assert.NotNil(t, m.CacheControl)
	assert.NotNil(t, m.RequestLogger)
	assert.Len(t, m.Middlewares(), 2)
}