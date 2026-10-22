package workspace_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/reearth/reearth-accounts/server/internal/admin/auth/session"
	workspacehandler "github.com/reearth/reearth-accounts/server/internal/admin/presentation/handler/workspace"
	"github.com/reearth/reearth-accounts/server/internal/admin/presentation/internal"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type transferEnv struct {
	op       *adminuser.AdminUser
	owner    *user.User
	member   *user.User
	ws       *workspace.Workspace
	wsRepo   workspace.Repo
	sess     *session.Manager
	serveReq func(t *testing.T, wsID, body string, cookie bool) *httptest.ResponseRecorder
}

func newTransferEnv(t *testing.T, members func(owner, member *user.User) map[workspace.UserID]workspace.Member, personal bool) *transferEnv {
	t.Helper()
	op := approvedAdmin("op@eukarya.io")
	owner := user.New().NewID().Name("Owner").Alias("owner").Email("owner@example.com").MustBuild()
	member := user.New().NewID().Name("Member").Alias("member").Email("member@example.com").MustBuild()
	w := workspace.New().NewID().Name("Team").Alias("team").Personal(personal).Members(members(owner, member)).MustBuild()
	env := &transferEnv{
		op:     op,
		owner:  owner,
		member: member,
		ws:     w,
		wsRepo: memory.NewWorkspaceWith(w),
		sess:   session.NewManager(testSecret, time.Hour),
	}
	e := newTestEchoWithUsers(env.wsRepo, memory.NewUserWith(owner, member), memory.NewAdminUserWith(op), env.sess)
	env.serveReq = func(t *testing.T, wsID, body string, cookie bool) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+wsID+"/transfer-owner", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if cookie {
			req.AddCookie(cookieFor(t, env.sess, op.ID()))
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}
	return env
}

func ownerAndWriter(owner, member *user.User) map[workspace.UserID]workspace.Member {
	return map[workspace.UserID]workspace.Member{
		owner.ID():  {Role: role.RoleOwner},
		member.ID(): {Role: role.RoleWriter},
	}
}

func transferBody(uid string) string {
	return `{"newOwnerUserId":"` + uid + `"}`
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) internal.ErrorResponse {
	t.Helper()
	var body internal.ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body
}

func TestTransferWorkspaceOwner_OK(t *testing.T) {
	env := newTransferEnv(t, ownerAndWriter, false)

	rec := env.serveReq(t, env.ws.ID().String(), transferBody(env.member.ID().String()), true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var body workspacehandler.TransferWorkspaceOwnerResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, workspacehandler.TransferredWorkspaceResponse{
		ID: env.ws.ID().String(), Name: "Team", Alias: "team", Personal: false,
	}, body.Workspace)
	assert.Equal(t, workspacehandler.WorkspaceMemberResponse{
		UserID: env.member.ID().String(), Name: "Member", Email: "member@example.com", Role: "owner",
	}, body.NewOwner)
	assert.Equal(t, []string{env.owner.ID().String()}, body.DemotedUserIDs)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	assert.Contains(t, raw, "demotedUserIds")
	assert.Contains(t, raw, "newOwner")
	assert.Contains(t, raw, "workspace")

	saved, err := env.wsRepo.FindByID(t.Context(), env.ws.ID())
	require.NoError(t, err)
	assert.Equal(t, role.RoleOwner, saved.Members().UserRole(env.member.ID()))
	assert.Equal(t, role.RoleMaintainer, saved.Members().UserRole(env.owner.ID()))
}

func TestTransferWorkspaceOwner_RetryAfterCommit_409(t *testing.T) {
	env := newTransferEnv(t, ownerAndWriter, false)

	first := env.serveReq(t, env.ws.ID().String(), transferBody(env.member.ID().String()), true)
	require.Equal(t, http.StatusOK, first.Code)

	rec := env.serveReq(t, env.ws.ID().String(), transferBody(env.member.ID().String()), true)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "already_owner", decodeError(t, rec).Code)
}

func TestTransferWorkspaceOwner_DomainErrors(t *testing.T) {
	tests := []struct {
		name     string
		members  func(owner, member *user.User) map[workspace.UserID]workspace.Member
		personal bool
		target   func(env *transferEnv) string
		status   int
		code     string
	}{
		{
			name:     "personal workspace",
			members:  ownerAndWriter,
			personal: true,
			target:   func(env *transferEnv) string { return env.member.ID().String() },
			status:   http.StatusUnprocessableEntity,
			code:     "personal_workspace",
		},
		{
			name: "target not member",
			members: func(owner, _ *user.User) map[workspace.UserID]workspace.Member {
				return map[workspace.UserID]workspace.Member{owner.ID(): {Role: role.RoleOwner}}
			},
			target: func(env *transferEnv) string { return env.member.ID().String() },
			status: http.StatusUnprocessableEntity,
			code:   "target_not_member",
		},
		{
			name: "target disabled",
			members: func(owner, member *user.User) map[workspace.UserID]workspace.Member {
				return map[workspace.UserID]workspace.Member{
					owner.ID():  {Role: role.RoleOwner},
					member.ID(): {Role: role.RoleWriter, Disabled: true},
				}
			},
			target: func(env *transferEnv) string { return env.member.ID().String() },
			status: http.StatusUnprocessableEntity,
			code:   "target_disabled",
		},
		{
			name: "target user record missing",
			members: func(owner, _ *user.User) map[workspace.UserID]workspace.Member {
				return map[workspace.UserID]workspace.Member{
					owner.ID():  {Role: role.RoleOwner},
					ghostUserID: {Role: role.RoleWriter},
				}
			},
			target: func(*transferEnv) string { return ghostUserID.String() },
			status: http.StatusUnprocessableEntity,
			code:   "target_user_not_found",
		},
		{
			name:    "target already sole owner",
			members: ownerAndWriter,
			target:  func(env *transferEnv) string { return env.owner.ID().String() },
			status:  http.StatusConflict,
			code:    "already_owner",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newTransferEnv(t, tt.members, tt.personal)
			rec := env.serveReq(t, env.ws.ID().String(), transferBody(tt.target(env)), true)
			assert.Equal(t, tt.status, rec.Code)
			assert.Equal(t, tt.code, decodeError(t, rec).Code)
		})
	}
}

var ghostUserID = user.NewID()

func TestTransferWorkspaceOwner_WorkspaceNotFound_404(t *testing.T) {
	env := newTransferEnv(t, ownerAndWriter, false)

	rec := env.serveReq(t, workspace.NewID().String(), transferBody(env.member.ID().String()), true)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestTransferWorkspaceOwner_BadRequest(t *testing.T) {
	env := newTransferEnv(t, ownerAndWriter, false)
	wsID := env.ws.ID().String()

	tests := []struct {
		name string
		wsID string
		body string
	}{
		{name: "invalid workspace id", wsID: "not-an-id", body: transferBody(env.member.ID().String())},
		{name: "malformed json", wsID: wsID, body: `{`},
		{name: "missing newOwnerUserId", wsID: wsID, body: `{}`},
		{name: "invalid newOwnerUserId", wsID: wsID, body: transferBody("not-an-id")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := env.serveReq(t, tt.wsID, tt.body, true)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestTransferWorkspaceOwner_Unauthorized_NoCookie(t *testing.T) {
	env := newTransferEnv(t, ownerAndWriter, false)

	rec := env.serveReq(t, env.ws.ID().String(), transferBody(env.member.ID().String()), false)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestTransferWorkspaceOwner_Forbidden_WhenNotApproved(t *testing.T) {
	pending := adminuser.New().NewID().Name("p@eukarya.io").Email("p@eukarya.io").Status(adminuser.StatusPending).MustBuild()
	owner := user.New().NewID().Name("Owner").Email("owner@example.com").MustBuild()
	member := user.New().NewID().Name("Member").Email("member@example.com").MustBuild()
	w := workspace.New().NewID().Name("Team").Alias("team").Members(ownerAndWriter(owner, member)).MustBuild()
	wsRepo := memory.NewWorkspaceWith(w)
	sess := session.NewManager(testSecret, time.Hour)
	e := newTestEchoWithUsers(wsRepo, memory.NewUserWith(owner, member), memory.NewAdminUserWith(pending), sess)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+w.ID().String()+"/transfer-owner", strings.NewReader(transferBody(member.ID().String())))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookieFor(t, sess, pending.ID()))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)

	saved, err := wsRepo.FindByID(t.Context(), w.ID())
	require.NoError(t, err)
	assert.Equal(t, role.RoleOwner, saved.Members().UserRole(owner.ID()))
}
