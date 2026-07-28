package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/admin/auth/session"
	"github.com/reearth/reearth-accounts/server/internal/admin/gateway/google"
	adminhttp "github.com/reearth/reearth-accounts/server/internal/admin/presentation"
	authhandler "github.com/reearth/reearth-accounts/server/internal/admin/presentation/handler/auth"
	mw "github.com/reearth/reearth-accounts/server/internal/admin/presentation/middleware"
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/authuc"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeVerifier struct{ claims *google.Claims }

func (f fakeVerifier) Verify(_ context.Context, _ string) (*google.Claims, error) {
	return f.claims, nil
}

type testValidator struct{ v *validator.Validate }

func (t testValidator) Validate(i any) error {
	if err := t.v.Struct(i); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return nil
}

func newTestEcho(t *testing.T, claims *google.Claims) *echo.Echo {
	t.Helper()
	repo := memory.NewAdminUser()
	signIn := authuc.NewGoogleSignInUseCase(repo, fakeVerifier{claims: claims}, authuc.GoogleSignInOptions{AllowedDomain: "eukarya.io"})
	getMe := authuc.NewGetMeUseCase(repo)
	sess := session.NewManager("test-secret-test-secret-test-secret", time.Hour)
	h := authhandler.NewHandler(signIn, getMe, sess, authhandler.CookieSecure(false))
	sessionMw := echo.MiddlewareFunc(mw.NewSessionMiddleware(sess))

	e := echo.New()
	e.Validator = testValidator{v: validator.New()}
	e.HTTPErrorHandler = adminhttp.CustomHTTPErrorHandler
	e.POST("/api/v1/auth/google", h.GoogleSignIn)
	e.POST("/api/v1/auth/logout", h.Logout) // public, mirrors the real router
	e.GET("/api/v1/me", h.Me, sessionMw)
	return e
}

func TestAuthFlow_GoogleThenMeThenLogout(t *testing.T) {
	e := newTestEcho(t, &google.Claims{Email: "alice@eukarya.io", EmailVerified: true, HD: "eukarya.io", Name: "Alice"})

	// 1. sign in
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/google", strings.NewReader(`{"id_token":"tok"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var signInBody authhandler.GoogleSignInResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &signInBody))
	assert.Equal(t, "pending", signInBody.Status)
	assert.Equal(t, "alice@eukarya.io", signInBody.Email)

	cookies := rec.Result().Cookies()
	var sessionCookie, csrfCookie *http.Cookie
	for _, c := range cookies {
		switch c.Name {
		case session.CookieName:
			sessionCookie = c
		case "admin_csrf":
			csrfCookie = c
		}
	}
	require.NotNil(t, sessionCookie, "session cookie must be set")
	assert.True(t, sessionCookie.HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, sessionCookie.SameSite)
	assert.NotEmpty(t, sessionCookie.Value)

	// The double-submit CSRF companion cookie must be set alongside the
	// session cookie, but readable by JS (not HttpOnly).
	require.NotNil(t, csrfCookie, "csrf cookie must be set")
	assert.False(t, csrfCookie.HttpOnly, "csrf cookie must be readable by JS")
	assert.False(t, csrfCookie.Secure, "csrf cookie Secure must match the handler's secure flag")
	assert.Equal(t, http.SameSiteLaxMode, csrfCookie.SameSite)
	assert.Equal(t, "/", csrfCookie.Path)
	assert.NotEmpty(t, csrfCookie.Value)
	// Both cookies share the same lifetime basis (the session TTL).
	assert.Equal(t, sessionCookie.MaxAge, csrfCookie.MaxAge)

	// 2. /me with the cookie
	meReq := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	meReq.AddCookie(sessionCookie)
	meRec := httptest.NewRecorder()
	e.ServeHTTP(meRec, meReq)
	require.Equal(t, http.StatusOK, meRec.Code)

	var me authhandler.MeResponse
	require.NoError(t, json.Unmarshal(meRec.Body.Bytes(), &me))
	assert.Equal(t, "alice@eukarya.io", me.Email)
	assert.Equal(t, "pending", me.Status)

	// 3. logout clears the cookie
	logoutReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	logoutReq.AddCookie(sessionCookie)
	logoutRec := httptest.NewRecorder()
	e.ServeHTTP(logoutRec, logoutReq)
	require.Equal(t, http.StatusNoContent, logoutRec.Code)

	var cleared, csrfCleared bool
	for _, c := range logoutRec.Result().Cookies() {
		if c.Name == session.CookieName && c.MaxAge < 0 {
			cleared = true
		}
		if c.Name == "admin_csrf" && c.MaxAge < 0 {
			csrfCleared = true
		}
	}
	assert.True(t, cleared, "logout must expire the session cookie")
	assert.True(t, csrfCleared, "logout must expire the csrf cookie")
}

func TestLogout_NoCookie_Succeeds(t *testing.T) {
	e := newTestEcho(t, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	// logout is public: it clears the cookie even with no/expired session
	assert.Equal(t, http.StatusNoContent, rec.Code)

	var cleared, csrfCleared bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == session.CookieName && c.MaxAge < 0 {
			cleared = true
		}
		if c.Name == "admin_csrf" && c.MaxAge < 0 {
			csrfCleared = true
		}
	}
	assert.True(t, cleared, "logout must expire the session cookie")
	assert.True(t, csrfCleared, "logout must expire the csrf cookie")
}

func TestMe_Unauthorized_NoCookie(t *testing.T) {
	e := newTestEcho(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestGoogleSignIn_DomainRejected(t *testing.T) {
	e := newTestEcho(t, &google.Claims{Email: "x@gmail.com", EmailVerified: true, HD: ""})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/google", strings.NewReader(`{"id_token":"tok"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestGoogleSignIn_MissingIDToken(t *testing.T) {
	e := newTestEcho(t, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/google", strings.NewReader(`{}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
