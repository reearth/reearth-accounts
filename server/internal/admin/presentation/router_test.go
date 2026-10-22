package presentation_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
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

type routerFixture struct {
	e      *echo.Echo
	repo   adminuser.Repo
	sess   *session.Manager
	op     *adminuser.AdminUser
	target *adminuser.AdminUser
}

// newRouterFixture registers the real admin routes. Only the handlers the tests
// call are constructed; the user/workspace handlers stay nil because their
// routes are never invoked here.
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
	e.HTTPErrorHandler = presentation.CustomHTTPErrorHandler
	presentation.RegisterRoutes(e, h)
	return &routerFixture{e: e, repo: repo, sess: sess, op: op, target: target}
}

func (f *routerFixture) do(t *testing.T, method, path, csrfCookie, csrfHeader string) *httptest.ResponseRecorder {
	t.Helper()
	tok, err := f.sess.Issue(f.op.ID(), time.Now())
	require.NoError(t, err)

	req := httptest.NewRequest(method, path, nil)
	req.AddCookie(&http.Cookie{Name: session.CookieName, Value: tok})
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

func TestRouter_AuthRoutesAreExemptFromCSRF(t *testing.T) {
	f := newRouterFixture(t)

	rec := f.do(t, http.MethodPost, "/api/v1/auth/logout", "", "")
	assert.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
}
