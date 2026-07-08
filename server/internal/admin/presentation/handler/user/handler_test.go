package user_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/admin/auth/session"
	adminpresentation "github.com/reearth/reearth-accounts/server/internal/admin/presentation"
	userhandler "github.com/reearth/reearth-accounts/server/internal/admin/presentation/handler/user"
	mw "github.com/reearth/reearth-accounts/server/internal/admin/presentation/middleware"
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/useruc"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-secret-test-secret-test-secret"

func approvedAdmin(email string) *adminuser.AdminUser {
	return adminuser.New().NewID().Name(email).Email(email).Status(adminuser.StatusApproved).MustBuild()
}

func usr(name, alias, email string) *user.User {
	return user.New().NewID().Name(name).Alias(alias).Email(email).MustBuild()
}

func newTestEcho(userRepo user.Repo, adminRepo adminuser.Repo, sess *session.Manager) *echo.Echo {
	h := userhandler.NewHandler(useruc.NewListUsersUseCase(userRepo))
	requireApproved := echo.MiddlewareFunc(mw.NewRequireApprovedMiddleware(sess, adminRepo))

	e := echo.New()
	e.HTTPErrorHandler = adminpresentation.CustomHTTPErrorHandler
	g := e.Group("/api/v1/users", requireApproved)
	g.GET("", h.ListUsers)
	return e
}

func cookieFor(t *testing.T, sess *session.Manager, id adminuser.ID) *http.Cookie {
	t.Helper()
	tok, err := sess.Issue(id, time.Now())
	require.NoError(t, err)
	return &http.Cookie{Name: session.CookieName, Value: tok}
}

func TestListUsers_OK(t *testing.T) {
	op := approvedAdmin("op@eukarya.io")
	adminRepo := memory.NewAdminUserWith(op)
	userRepo := memory.NewUserWith(usr("Alpha", "alpha", "alpha@example.com"), usr("Beta", "beta", "beta@example.com"))
	sess := session.NewManager(testSecret, time.Hour)
	e := newTestEcho(userRepo, adminRepo, sess)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.AddCookie(cookieFor(t, sess, op.ID()))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var body userhandler.ListUsersResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, int64(2), body.TotalCount)
	assert.Len(t, body.Items, 2)
}

func TestListUsers_Keyword(t *testing.T) {
	op := approvedAdmin("op@eukarya.io")
	adminRepo := memory.NewAdminUserWith(op)
	userRepo := memory.NewUserWith(usr("Alpha", "alpha", "alpha@example.com"), usr("Beta", "beta", "beta@example.com"))
	sess := session.NewManager(testSecret, time.Hour)
	e := newTestEcho(userRepo, adminRepo, sess)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users?q=beta", nil)
	req.AddCookie(cookieFor(t, sess, op.ID()))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var body userhandler.ListUsersResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, int64(1), body.TotalCount)
	require.Len(t, body.Items, 1)
	assert.Equal(t, "Beta", body.Items[0].Name)
}

func TestListUsers_InvalidPagination(t *testing.T) {
	cases := []struct {
		name  string
		query string
	}{
		{name: "page not a number", query: "page=abc"},
		{name: "page zero", query: "page=0"},
		{name: "page negative", query: "page=-1"},
		{name: "per_page not a number", query: "per_page=abc"},
		{name: "per_page zero", query: "per_page=0"},
		{name: "page above upper bound", query: "page=1000000000001"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			op := approvedAdmin("op@eukarya.io")
			adminRepo := memory.NewAdminUserWith(op)
			userRepo := memory.NewUserWith(usr("A", "a", "a@example.com"))
			sess := session.NewManager(testSecret, time.Hour)
			e := newTestEcho(userRepo, adminRepo, sess)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/users?"+tc.query, nil)
			req.AddCookie(cookieFor(t, sess, op.ID()))
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}
