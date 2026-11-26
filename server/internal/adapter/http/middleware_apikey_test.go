package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/adapter"
	httpinternal "github.com/reearth/reearth-accounts/server/internal/adapter/http/internal"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
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

func TestAPIKeyOrAuth_RefusesClientCalls(t *testing.T) {
	u := user.New().NewID().Name("u").Email("u@example.com").MustBuild()
	uid := u.ID()
	iid := id.NewIntegrationID()
	svc := &workspace.Client{ID: "svc", Name: "svc"}
	ok := func(c echo.Context) error { return c.String(http.StatusOK, "ok") }
	call := func(ctx context.Context, authHeader string) int {
		req := httptest.NewRequest(http.MethodPost, "/", nil).WithContext(ctx)
		if authHeader != "" {
			req.Header.Set(echo.HeaderAuthorization, authHeader)
		}
		rec := httptest.NewRecorder()
		c := echo.New().NewContext(req, rec)
		if err := APIKeyOrAuth("secret")(ok)(c); err != nil {
			httpinternal.CustomHTTPErrorHandler(err, c)
		}
		return rec.Code
	}
	bg := context.Background()
	actingForUser := (&adapter.ClientCall{
		Client: svc, Subject: workspace.UserSubject(uid), User: u,
		Operator: &workspace.Operator{User: &uid, Client: svc},
	}).Attach(bg)
	actingForIntegration := (&adapter.ClientCall{
		Client: svc, Subject: workspace.IntegrationSubject(iid),
		Operator: &workspace.Operator{Integration: &iid, Client: svc},
	}).Attach(bg)
	plainUser := adapter.AttachOperator(adapter.AttachUser(bg, u), &workspace.Operator{User: &uid})

	assert.Equal(t, http.StatusForbidden, call(actingForUser, ""), "a client acting for a user")
	assert.Equal(t, http.StatusForbidden, call(actingForIntegration, ""), "a client acting for an integration")
	assert.Equal(t, http.StatusOK, call(plainUser, ""), "a user's own token")
	assert.Equal(t, http.StatusOK, call(bg, bearerPrefix+"secret"), "the static key")
}
