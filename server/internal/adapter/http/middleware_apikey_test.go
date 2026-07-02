package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	httpinternal "github.com/reearth/reearth-accounts/server/internal/adapter/http/internal"
	"github.com/stretchr/testify/assert"
)

func TestSkipJWTOnAPIKey(t *testing.T) {
	ok := func(c echo.Context) error { return c.String(http.StatusOK, "ok") }

	jwtCalled := false
	jwtMW := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			jwtCalled = true
			return next(c)
		}
	}

	newCtx := func(token string) echo.Context {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if token != "" {
			req.Header.Set(echo.HeaderAuthorization, bearerPrefix+token)
		}
		return e.NewContext(req, httptest.NewRecorder())
	}

	t.Run("nil jwt returns nil", func(t *testing.T) {
		assert.Nil(t, skipJWTOnAPIKey(nil, "key"))
	})

	t.Run("no keys returns jwt directly", func(t *testing.T) {
		jwtCalled = false
		m := skipJWTOnAPIKey(jwtMW)
		_ = m(ok)(newCtx("anything"))
		assert.True(t, jwtCalled)
	})

	t.Run("all empty keys returns jwt directly", func(t *testing.T) {
		jwtCalled = false
		m := skipJWTOnAPIKey(jwtMW, "", "")
		_ = m(ok)(newCtx("anything"))
		assert.True(t, jwtCalled)
	})

	t.Run("token matches first key skips JWT", func(t *testing.T) {
		jwtCalled = false
		m := skipJWTOnAPIKey(jwtMW, "key1", "key2")
		_ = m(ok)(newCtx("key1"))
		assert.False(t, jwtCalled)
	})

	t.Run("token matches second key skips JWT", func(t *testing.T) {
		jwtCalled = false
		m := skipJWTOnAPIKey(jwtMW, "key1", "key2")
		_ = m(ok)(newCtx("key2"))
		assert.False(t, jwtCalled)
	})

	t.Run("unrecognized token runs JWT", func(t *testing.T) {
		jwtCalled = false
		m := skipJWTOnAPIKey(jwtMW, "key1", "key2")
		_ = m(ok)(newCtx("unknown"))
		assert.True(t, jwtCalled)
	})

	t.Run("no authorization header runs JWT", func(t *testing.T) {
		jwtCalled = false
		m := skipJWTOnAPIKey(jwtMW, "key1")
		_ = m(ok)(newCtx(""))
		assert.True(t, jwtCalled)
	})
}

func TestAPIKeyOrAuth(t *testing.T) {
	ok := func(c echo.Context) error { return c.String(http.StatusOK, "ok") }

	call := func(cfgKey, authHeader string) int {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if authHeader != "" {
			req.Header.Set(echo.HeaderAuthorization, authHeader)
		}
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		mw := APIKeyOrAuth(cfgKey)
		if err := mw(ok)(c); err != nil {
			httpinternal.CustomHTTPErrorHandler(err, c)
		}
		return rec.Code
	}

	t.Run("no key configured rejects request", func(t *testing.T) {
		assert.Equal(t, http.StatusUnauthorized, call("", bearerPrefix+"anything"))
	})

	t.Run("correct key is accepted", func(t *testing.T) {
		assert.Equal(t, http.StatusOK, call("secret", bearerPrefix+"secret"))
	})

	t.Run("wrong key is rejected", func(t *testing.T) {
		assert.Equal(t, http.StatusUnauthorized, call("secret", bearerPrefix+"wrong"))
	})

	t.Run("no authorization header is rejected", func(t *testing.T) {
		assert.Equal(t, http.StatusUnauthorized, call("secret", ""))
	})

	t.Run("token without Bearer prefix is rejected", func(t *testing.T) {
		assert.Equal(t, http.StatusUnauthorized, call("secret", "secret"))
	})
}