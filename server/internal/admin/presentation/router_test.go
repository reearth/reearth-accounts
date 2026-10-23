package presentation_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/reearth/reearth-accounts/server/internal/admin/auth/session"
	"github.com/reearth/reearth-accounts/server/internal/admin/presentation"
	adminuserhandler "github.com/reearth/reearth-accounts/server/internal/admin/presentation/handler/adminuser"
	"github.com/reearth/reearth-accounts/server/internal/admin/presentation/handler/auth"
	"github.com/reearth/reearth-accounts/server/internal/admin/presentation/internal"
	mw "github.com/reearth/reearth-accounts/server/internal/admin/presentation/middleware"
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/adminuseruc"
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/authz"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testSecret = "test-secret-test-secret-test-secret"
	csrfToken  = "csrf-token-value"
)

type testValidator struct{ v *validator.Validate }

func (t testValidator) Validate(i any) error {
	if err := t.v.Struct(i); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return nil
}

type routerFixture struct {
	e      *echo.Echo
	repo   adminuser.Repo
	sess   *session.Manager
	op     *adminuser.AdminUser
	target *adminuser.AdminUser
}

// newRouterFixture registers the real admin routes. Only the admin-user handler
// is constructed; the user/workspace handlers stay nil, so a request must not
// get past the middleware on their routes.
func newRouterFixture(t *testing.T) *routerFixture {
	t.Helper()
	op := adminuser.New().NewID().Name("op").Email("op@eukarya.io").
		Status(adminuser.StatusApproved).Role(adminuser.RoleSystemAdmin).MustBuild()
	target := adminuser.New().NewID().Name("new").Email("new@eukarya.io").
		Status(adminuser.StatusPending).MustBuild()
	repo := memory.NewAdminUserWith(op, target)
	sess := session.NewManager(testSecret, time.Hour)

	h := presentation.NewHandler(
		adminuserhandler.NewHandler(
			adminuseruc.NewListAdminUsersUseCase(repo),
			adminuseruc.NewApproveAdminUserUseCase(repo),
			adminuseruc.NewRejectAdminUserUseCase(repo),
			adminuseruc.NewSetRoleUseCase(repo),
		),
		auth.NewHandler(nil, nil, sess, false),
		nil,
		nil,
		mw.NewSessionMiddleware(sess),
		mw.NewRequireApprovedMiddleware(sess, repo),
		authz.NewChecker(nil),
	)

	e := echo.New()
	// A mutating user/workspace route that skips RequireCSRF reaches a nil
	// handler; Recover turns that panic into a 500 so the route walk fails
	// with the route's method and path.
	e.Use(echomw.Recover())
	e.HTTPErrorHandler = presentation.CustomHTTPErrorHandler
	e.Validator = testValidator{v: validator.New()}
	presentation.RegisterRoutes(e, h)
	return &routerFixture{e: e, repo: repo, sess: sess, op: op, target: target}
}

func (f *routerFixture) do(t *testing.T, method, path, csrfCookie, csrfHeader string) *httptest.ResponseRecorder {
	t.Helper()
	tok, err := f.sess.Issue(f.op.ID(), time.Now())
	require.NoError(t, err)

	req := httptest.NewRequest(method, path, nil)
	req.AddCookie(&http.Cookie{Name: session.CookieName, Value: tok})
	return f.serve(req, csrfCookie, csrfHeader)
}

// serve sends req with the given CSRF cookie/header (each omitted when empty)
// and no session cookie unless the caller already added one.
func (f *routerFixture) serve(req *http.Request, csrfCookie, csrfHeader string) *httptest.ResponseRecorder {
	if csrfCookie != "" {
		req.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: csrfCookie})
	}
	if csrfHeader != "" {
		req.Header.Set(session.CSRFHeaderName, csrfHeader)
	}
	rec := httptest.NewRecorder()
	f.e.ServeHTTP(rec, req)
	return rec
}

func TestRouter_MutatingRoutesRequireCSRF(t *testing.T) {
	tests := []struct {
		name    string
		cookie  string
		header  string
		wantMsg string
	}{
		{name: "no header", cookie: csrfToken, wantMsg: "missing csrf token"},
		{name: "no cookie", header: csrfToken, wantMsg: "missing csrf token"},
		{name: "mismatched header", cookie: csrfToken, header: "forged", wantMsg: "invalid csrf token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newRouterFixture(t)
			for _, path := range []string{
				"/api/v1/admin-users/" + f.target.ID().String() + "/approve",
				"/api/v1/admin-users/" + f.target.ID().String() + "/reject",
			} {
				rec := f.do(t, http.MethodPost, path, tt.cookie, tt.header)
				require.Equal(t, http.StatusForbidden, rec.Code, path)

				var body internal.ErrorResponse
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
				assert.Equal(t, "Forbidden", body.Code)
				assert.Equal(t, tt.wantMsg, body.Message)
			}

			rec := f.do(t, http.MethodPut, "/api/v1/admin-users/"+f.target.ID().String()+"/roles", tt.cookie, tt.header)
			assert.Equal(t, http.StatusForbidden, rec.Code)

			got, err := f.repo.FindByID(context.Background(), f.target.ID())
			require.NoError(t, err)
			assert.Equal(t, adminuser.StatusPending, got.Status(), "a rejected CSRF check must not reach the handler")
		})
	}
}

func TestRouter_MutatingRouteWithMatchingCSRFPasses(t *testing.T) {
	f := newRouterFixture(t)

	rec := f.do(t, http.MethodPost, "/api/v1/admin-users/"+f.target.ID().String()+"/approve", csrfToken, csrfToken)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got, err := f.repo.FindByID(context.Background(), f.target.ID())
	require.NoError(t, err)
	assert.Equal(t, adminuser.StatusApproved, got.Status())
}

func TestRouter_SafeMethodsSkipCSRF(t *testing.T) {
	f := newRouterFixture(t)

	rec := f.do(t, http.MethodGet, "/api/v1/admin-users", "", "")
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// Unauthenticated mutating requests must get 401 before the CSRF check runs,
// so session expiry is reported the same way on every route.
func TestRouter_UnauthenticatedMutatingRouteIs401(t *testing.T) {
	f := newRouterFixture(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin-users/"+f.target.ID().String()+"/approve", nil)
	rec := f.serve(req, "", "")
	assert.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
}

var routeParam = regexp.MustCompile(`:[^/]+`)

// Walks every registered route so a mutating route added to a group that does
// not carry RequireCSRF fails here instead of shipping unprotected.
func TestRouter_AllMutatingRoutesRequireCSRF(t *testing.T) {
	f := newRouterFixture(t)
	exempt := map[string]bool{
		http.MethodPost + " /api/v1/auth/google": true,
		http.MethodPost + " /api/v1/auth/logout": true,
	}

	checked := 0
	for _, r := range f.e.Routes() {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions, echo.RouteNotFound:
			continue
		}
		if !strings.HasPrefix(r.Path, "/api/v1/") || exempt[r.Method+" "+r.Path] {
			continue
		}
		path := routeParam.ReplaceAllString(r.Path, f.target.ID().String())

		rec := f.do(t, r.Method, path, "", "")
		require.Equal(t, http.StatusForbidden, rec.Code, "%s %s: %s", r.Method, r.Path, rec.Body.String())
		var body internal.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "missing csrf token", body.Message, "%s %s", r.Method, r.Path)
		checked++
	}
	assert.GreaterOrEqual(t, checked, 3, "expected at least the admin-user approve/reject/roles routes")
}

func TestRouter_SignInIsExemptFromCSRF(t *testing.T) {
	f := newRouterFixture(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/google", strings.NewReader("{}"))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := f.serve(req, "", "")
	assert.Equal(t, http.StatusBadRequest, rec.Code, "an empty body must fail validation, not the CSRF check: %s", rec.Body.String())
}

func TestRouter_Logout(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		cookie   string
		header   string
		wantCode int
	}{
		// A cross-site POST looks like this too: SameSite=Lax keeps admin_csrf
		// off it, so this guard does not reject it.
		{name: "no csrf cookie clears stale session", path: "/api/v1/auth/logout", wantCode: http.StatusNoContent},
		{name: "matching token", path: "/api/v1/auth/logout", cookie: csrfToken, header: csrfToken, wantCode: http.StatusNoContent},
		{name: "same-site forged request carrying the cookie without header", path: "/api/v1/auth/logout", cookie: csrfToken, wantCode: http.StatusForbidden},
		{name: "mismatched header", path: "/api/v1/auth/logout", cookie: csrfToken, header: "forged", wantCode: http.StatusForbidden},
		{name: "trailing slash is not the exempt route", path: "/api/v1/auth/logout/", wantCode: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newRouterFixture(t)

			rec := f.serve(httptest.NewRequest(http.MethodPost, tt.path, nil), tt.cookie, tt.header)
			assert.Equal(t, tt.wantCode, rec.Code, rec.Body.String())
			if tt.wantCode != http.StatusNoContent {
				assert.Empty(t, rec.Result().Cookies(), "a rejected logout must not clear cookies")
			}
		})
	}
}
