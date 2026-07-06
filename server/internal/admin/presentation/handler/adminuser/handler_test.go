package adminuser_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/admin/auth/session"
	adminpresentation "github.com/reearth/reearth-accounts/server/internal/admin/presentation"
	adminuserhandler "github.com/reearth/reearth-accounts/server/internal/admin/presentation/handler/adminuser"
	mw "github.com/reearth/reearth-accounts/server/internal/admin/presentation/middleware"
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/adminuseruc"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-secret-test-secret-test-secret"

func approvedUser(email string) *adminuser.AdminUser {
	return adminuser.New().NewID().Name(email).Email(email).Status(adminuser.StatusApproved).MustBuild()
}
func pendingUser(email string) *adminuser.AdminUser {
	return adminuser.New().NewID().Name(email).Email(email).Status(adminuser.StatusPending).MustBuild()
}

func newTestEcho(repo adminuser.Repo, sess *session.Manager) *echo.Echo {
	h := adminuserhandler.NewHandler(
		adminuseruc.NewListAdminUsersUseCase(repo),
		adminuseruc.NewApproveAdminUserUseCase(repo),
		adminuseruc.NewRejectAdminUserUseCase(repo),
	)
	requireApproved := echo.MiddlewareFunc(mw.NewRequireApprovedMiddleware(sess, repo))

	e := echo.New()
	e.HTTPErrorHandler = adminpresentation.CustomHTTPErrorHandler
	g := e.Group("/api/v1/admin-users", requireApproved)
	g.GET("", h.ListAdminUsers)
	g.POST("/:id/approve", h.ApproveAdminUser)
	g.POST("/:id/reject", h.RejectAdminUser)
	return e
}

func cookieFor(t *testing.T, sess *session.Manager, id adminuser.ID) *http.Cookie {
	t.Helper()
	tok, err := sess.Issue(id, time.Now())
	require.NoError(t, err)
	return &http.Cookie{Name: session.CookieName, Value: tok}
}

func TestListAdminUsers_OK(t *testing.T) {
	op := approvedUser("op@eukarya.io")
	target := pendingUser("new@eukarya.io")
	repo := memory.NewAdminUserWith(op, target)
	sess := session.NewManager(testSecret, time.Hour)
	e := newTestEcho(repo, sess)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin-users", nil)
	req.AddCookie(cookieFor(t, sess, op.ID()))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var body adminuserhandler.ListAdminUsersResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, int64(2), body.TotalCount)
	assert.Len(t, body.Items, 2)
}

func TestListAdminUsers_Unauthorized_NoCookie(t *testing.T) {
	repo := memory.NewAdminUserWith(approvedUser("op@eukarya.io"))
	sess := session.NewManager(testSecret, time.Hour)
	e := newTestEcho(repo, sess)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin-users", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAdminUsers_Forbidden_WhenNotApproved(t *testing.T) {
	pendingOp := pendingUser("pending@eukarya.io")
	repo := memory.NewAdminUserWith(pendingOp)
	sess := session.NewManager(testSecret, time.Hour)
	e := newTestEcho(repo, sess)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin-users", nil)
	req.AddCookie(cookieFor(t, sess, pendingOp.ID()))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestApproveAdminUser_OK(t *testing.T) {
	op := approvedUser("op@eukarya.io")
	target := pendingUser("new@eukarya.io")
	repo := memory.NewAdminUserWith(op, target)
	sess := session.NewManager(testSecret, time.Hour)
	e := newTestEcho(repo, sess)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin-users/"+target.ID().String()+"/approve", nil)
	req.AddCookie(cookieFor(t, sess, op.ID()))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var body adminuserhandler.AdminUserResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "approved", body.Status)
	assert.Equal(t, op.ID().String(), body.ApprovedBy)
}

func TestApproveAdminUser_CannotApproveSelf(t *testing.T) {
	op := approvedUser("op@eukarya.io")
	repo := memory.NewAdminUserWith(op)
	sess := session.NewManager(testSecret, time.Hour)
	e := newTestEcho(repo, sess)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin-users/"+op.ID().String()+"/approve", nil)
	req.AddCookie(cookieFor(t, sess, op.ID()))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRejectAdminUser_InvalidID(t *testing.T) {
	op := approvedUser("op@eukarya.io")
	repo := memory.NewAdminUserWith(op)
	sess := session.NewManager(testSecret, time.Hour)
	e := newTestEcho(repo, sess)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin-users/not-an-id/reject", nil)
	req.AddCookie(cookieFor(t, sess, op.ID()))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
