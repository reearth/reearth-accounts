package workspace_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/admin/auth/session"
	adminpresentation "github.com/reearth/reearth-accounts/server/internal/admin/presentation"
	workspacehandler "github.com/reearth/reearth-accounts/server/internal/admin/presentation/handler/workspace"
	mw "github.com/reearth/reearth-accounts/server/internal/admin/presentation/middleware"
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/workspaceuc"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-secret-test-secret-test-secret"

func approvedAdmin(email string) *adminuser.AdminUser {
	return adminuser.New().NewID().Name(email).Email(email).Status(adminuser.StatusApproved).MustBuild()
}

func ws(name, alias string) *workspace.Workspace {
	return workspace.New().NewID().Name(name).Alias(alias).MustBuild()
}

func newTestEcho(wsRepo workspace.Repo, adminRepo adminuser.Repo, sess *session.Manager) *echo.Echo {
	return newTestEchoWithUsers(wsRepo, memory.NewUserWith(), adminRepo, sess)
}

func newTestEchoWithUsers(wsRepo workspace.Repo, userRepo user.Repo, adminRepo adminuser.Repo, sess *session.Manager) *echo.Echo {
	h := workspacehandler.NewHandler(
		workspaceuc.NewGetWorkspaceUseCase(wsRepo),
		workspaceuc.NewListWorkspacesUseCase(wsRepo),
		workspaceuc.NewListWorkspaceMembersUseCase(wsRepo, userRepo),
	)
	requireApproved := echo.MiddlewareFunc(mw.NewRequireApprovedMiddleware(sess, adminRepo))

	e := echo.New()
	e.HTTPErrorHandler = adminpresentation.CustomHTTPErrorHandler
	g := e.Group("/api/v1/workspaces", requireApproved)
	g.GET("", h.ListWorkspaces)
	g.GET("/:id", h.GetWorkspace)
	g.GET("/:id/members", h.GetWorkspaceMembers)
	return e
}

func cookieFor(t *testing.T, sess *session.Manager, id adminuser.ID) *http.Cookie {
	t.Helper()
	tok, err := sess.Issue(id, time.Now())
	require.NoError(t, err)
	return &http.Cookie{Name: session.CookieName, Value: tok}
}

func TestListWorkspaces_OK(t *testing.T) {
	op := approvedAdmin("op@eukarya.io")
	adminRepo := memory.NewAdminUserWith(op)
	wsRepo := memory.NewWorkspaceWith(ws("Alpha", "alpha"), ws("Beta", "beta"))
	sess := session.NewManager(testSecret, time.Hour)
	e := newTestEcho(wsRepo, adminRepo, sess)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces", nil)
	req.AddCookie(cookieFor(t, sess, op.ID()))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var body workspacehandler.ListWorkspacesResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, int64(2), body.TotalCount)
	assert.Len(t, body.Items, 2)
}

func TestListWorkspaces_Keyword(t *testing.T) {
	op := approvedAdmin("op@eukarya.io")
	adminRepo := memory.NewAdminUserWith(op)
	wsRepo := memory.NewWorkspaceWith(ws("Alpha", "alpha"), ws("Beta", "beta"))
	sess := session.NewManager(testSecret, time.Hour)
	e := newTestEcho(wsRepo, adminRepo, sess)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces?q=beta", nil)
	req.AddCookie(cookieFor(t, sess, op.ID()))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var body workspacehandler.ListWorkspacesResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, int64(1), body.TotalCount)
	require.Len(t, body.Items, 1)
	assert.Equal(t, "Beta", body.Items[0].Name)
}

func TestListWorkspaces_InvalidPagination(t *testing.T) {
	op := approvedAdmin("op@eukarya.io")
	adminRepo := memory.NewAdminUserWith(op)
	wsRepo := memory.NewWorkspaceWith(ws("A", "a"))
	sess := session.NewManager(testSecret, time.Hour)
	e := newTestEcho(wsRepo, adminRepo, sess)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces?page=abc", nil)
	req.AddCookie(cookieFor(t, sess, op.ID()))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestListWorkspaces_Unauthorized_NoCookie(t *testing.T) {
	adminRepo := memory.NewAdminUserWith(approvedAdmin("op@eukarya.io"))
	wsRepo := memory.NewWorkspaceWith(ws("A", "a"))
	sess := session.NewManager(testSecret, time.Hour)
	e := newTestEcho(wsRepo, adminRepo, sess)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestListWorkspaces_Forbidden_WhenNotApproved(t *testing.T) {
	pending := adminuser.New().NewID().Name("p@eukarya.io").Email("p@eukarya.io").Status(adminuser.StatusPending).MustBuild()
	adminRepo := memory.NewAdminUserWith(pending)
	wsRepo := memory.NewWorkspaceWith(ws("A", "a"))
	sess := session.NewManager(testSecret, time.Hour)
	e := newTestEcho(wsRepo, adminRepo, sess)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces", nil)
	req.AddCookie(cookieFor(t, sess, pending.ID()))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestGetWorkspace_OK(t *testing.T) {
	op := approvedAdmin("op@eukarya.io")
	adminRepo := memory.NewAdminUserWith(op)
	w := ws("Alpha", "alpha")
	wsRepo := memory.NewWorkspaceWith(w)
	sess := session.NewManager(testSecret, time.Hour)
	e := newTestEcho(wsRepo, adminRepo, sess)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+w.ID().String(), nil)
	req.AddCookie(cookieFor(t, sess, op.ID()))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var body workspacehandler.WorkspaceResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, w.ID().String(), body.ID)
	assert.Equal(t, "Alpha", body.Name)
}

func TestGetWorkspace_InvalidID(t *testing.T) {
	op := approvedAdmin("op@eukarya.io")
	adminRepo := memory.NewAdminUserWith(op)
	wsRepo := memory.NewWorkspaceWith(ws("A", "a"))
	sess := session.NewManager(testSecret, time.Hour)
	e := newTestEcho(wsRepo, adminRepo, sess)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/not-an-id", nil)
	req.AddCookie(cookieFor(t, sess, op.ID()))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetWorkspace_NotFound(t *testing.T) {
	op := approvedAdmin("op@eukarya.io")
	adminRepo := memory.NewAdminUserWith(op)
	wsRepo := memory.NewWorkspaceWith(ws("A", "a"))
	sess := session.NewManager(testSecret, time.Hour)
	e := newTestEcho(wsRepo, adminRepo, sess)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+workspace.NewID().String(), nil)
	req.AddCookie(cookieFor(t, sess, op.ID()))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestGetWorkspaceMembers_OK(t *testing.T) {
	op := approvedAdmin("op@eukarya.io")
	adminRepo := memory.NewAdminUserWith(op)

	u := user.New().NewID().Name("Alice").Alias("alice").Email("alice@example.com").MustBuild()
	w := workspace.New().NewID().Name("Team").Alias("team").Members(map[workspace.UserID]workspace.Member{
		u.ID(): {Role: role.RoleOwner},
	}).MustBuild()
	wsRepo := memory.NewWorkspaceWith(w)
	userRepo := memory.NewUserWith(u)
	sess := session.NewManager(testSecret, time.Hour)
	e := newTestEchoWithUsers(wsRepo, userRepo, adminRepo, sess)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+w.ID().String()+"/members", nil)
	req.AddCookie(cookieFor(t, sess, op.ID()))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var body []workspacehandler.WorkspaceMemberResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body, 1)
	assert.Equal(t, u.ID().String(), body[0].UserID)
	assert.Equal(t, "Alice", body[0].Name)
	assert.Equal(t, "alice@example.com", body[0].Email)
	assert.Equal(t, "owner", body[0].Role)
}

func TestGetWorkspaceMembers_NotFound(t *testing.T) {
	op := approvedAdmin("op@eukarya.io")
	adminRepo := memory.NewAdminUserWith(op)
	wsRepo := memory.NewWorkspaceWith(ws("A", "a"))
	sess := session.NewManager(testSecret, time.Hour)
	e := newTestEcho(wsRepo, adminRepo, sess)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+workspace.NewID().String()+"/members", nil)
	req.AddCookie(cookieFor(t, sess, op.ID()))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
