package presentation_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cerbos/cerbos-sdk-go/cerbos"
	effectv1 "github.com/cerbos/cerbos/api/genpb/cerbos/effect/v1"
	requestv1 "github.com/cerbos/cerbos/api/genpb/cerbos/request/v1"
	responsev1 "github.com/cerbos/cerbos/api/genpb/cerbos/response/v1"
	svcv1 "github.com/cerbos/cerbos/api/genpb/cerbos/svc/v1"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/reearth/reearth-accounts/server/internal/admin/auth/session"
	"github.com/reearth/reearth-accounts/server/internal/admin/presentation"
	adminuserhandler "github.com/reearth/reearth-accounts/server/internal/admin/presentation/handler/adminuser"
	"github.com/reearth/reearth-accounts/server/internal/admin/presentation/handler/auth"
	workspacehandler "github.com/reearth/reearth-accounts/server/internal/admin/presentation/handler/workspace"
	"github.com/reearth/reearth-accounts/server/internal/admin/presentation/internal"
	mw "github.com/reearth/reearth-accounts/server/internal/admin/presentation/middleware"
	adminrbac "github.com/reearth/reearth-accounts/server/internal/admin/rbac"
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/adminuseruc"
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/authz"
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/workspaceuc"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/cerbos/generator"
	"github.com/reearth/reearthx/usecasex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// policyCerbos answers CheckResources from the rules compiled into the
// accounts-admin policy, so a route test exercises the real Checker, the real
// route table and the real role matrix without a Cerbos container.
type policyCerbos struct {
	svcv1.UnimplementedCerbosServiceServer
	roles map[string][]string // "<kind>/<action>" -> roles
}

func newPolicyCerbos() *policyCerbos {
	p := &policyCerbos{roles: map[string][]string{}}
	for _, r := range adminrbac.DefineResources(generator.NewResourceBuilder(adminrbac.ServiceName)) {
		for _, a := range r.Actions {
			p.roles[r.Resource+"/"+a.Action] = a.Roles
		}
	}
	return p
}

func (p *policyCerbos) CheckResources(_ context.Context, req *requestv1.CheckResourcesRequest) (*responsev1.CheckResourcesResponse, error) {
	res := &responsev1.CheckResourcesResponse{RequestId: req.GetRequestId()}
	for _, entry := range req.GetResources() {
		r := entry.GetResource()
		actions := map[string]effectv1.Effect{}
		for _, action := range entry.GetActions() {
			effect := effectv1.Effect_EFFECT_DENY
			allowed := p.roles[r.GetKind()+"/"+action]
			for _, pr := range req.GetPrincipal().GetRoles() {
				if slices.Contains(allowed, pr) {
					effect = effectv1.Effect_EFFECT_ALLOW
				}
			}
			actions[action] = effect
		}
		res.Results = append(res.Results, &responsev1.CheckResourcesResponse_ResultEntry{
			Resource: &responsev1.CheckResourcesResponse_ResultEntry_Resource{Id: r.GetId(), Kind: r.GetKind()},
			Actions:  actions,
		})
	}
	return res, nil
}

func startPolicyCerbos(t *testing.T) *cerbos.GRPCClient {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := grpc.NewServer()
	svcv1.RegisterCerbosServiceServer(srv, newPolicyCerbos())
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	client, err := cerbos.New("passthrough:///"+lis.Addr().String(), cerbos.WithPlaintext())
	require.NoError(t, err)
	return client
}

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

// newRouterFixture registers the real admin routes. Only the handlers the tests
// call are constructed; the user/workspace handlers stay nil because their
// routes are never invoked here. Recover turns a request that does reach a nil
// handler into a 500, so the route walk fails naming that route instead of
// panicking.
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
	e.Use(middleware.Recover())
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

	checked := map[string]bool{}
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
		checked[r.Method+" "+r.Path] = true
	}
	for _, want := range []string{
		http.MethodPost + " /api/v1/admin-users/:id/approve",
		http.MethodPost + " /api/v1/admin-users/:id/reject",
		http.MethodPut + " /api/v1/admin-users/:id/roles",
		http.MethodPost + " /api/v1/workspaces/:id/transfer-owner",
	} {
		assert.True(t, checked[want], "route walk did not reach %s", want)
	}
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
		{name: "no csrf cookie clears stale session", path: "/api/v1/auth/logout", wantCode: http.StatusNoContent},
		{name: "matching token", path: "/api/v1/auth/logout", cookie: csrfToken, header: csrfToken, wantCode: http.StatusNoContent},
		{name: "cookie without header", path: "/api/v1/auth/logout", cookie: csrfToken, wantCode: http.StatusForbidden},
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

func TestRoutes_TransferWorkspaceOwner_ByRole(t *testing.T) {
	tests := []struct {
		name    string
		role    adminuser.Role
		status  int
		wantMsg string
	}{
		// The CSRF token matches, so the 403 must come from the role check.
		{name: "viewer is forbidden", role: adminuser.RoleViewer, status: http.StatusForbidden, wantMsg: "forbidden"},
		{name: "system admin is allowed", role: adminuser.RoleSystemAdmin, status: http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := adminuser.New().NewID().Name("op@eukarya.io").Email("op@eukarya.io").
				Status(adminuser.StatusApproved).Role(tt.role).MustBuild()
			owner := user.New().NewID().Name("Owner").Email("owner@example.com").MustBuild()
			member := user.New().NewID().Name("Member").Email("member@example.com").MustBuild()
			w := workspace.New().NewID().Name("Team").Alias("team").Members(map[workspace.UserID]workspace.Member{
				owner.ID():  {Role: role.RoleOwner},
				member.ID(): {Role: role.RoleWriter},
			}).MustBuild()
			wsRepo := memory.NewWorkspaceWith(w)
			userRepo := memory.NewUserWith(owner, member)
			roles := make([]*role.Role, 0, 2)
			for _, rt := range []role.RoleType{role.RoleOwner, role.RoleMaintainer} {
				roles = append(roles, role.New().NewID().Name(string(rt)).MustBuild())
			}
			sess := session.NewManager(testSecret, time.Hour)

			h := &presentation.Handler{
				Workspace: workspacehandler.NewHandler(
					workspaceuc.NewGetWorkspaceUseCase(wsRepo),
					workspaceuc.NewListWorkspacesUseCase(wsRepo),
					workspaceuc.NewListWorkspaceMembersUseCase(wsRepo, userRepo),
					workspaceuc.NewTransferOwnerUseCase(wsRepo, userRepo, memory.NewRoleWith(roles...), memory.NewPermittable(), &usecasex.NopTransaction{}),
				),
				SessionMw:       mw.NewSessionMiddleware(sess),
				RequireApproved: mw.NewRequireApprovedMiddleware(sess, memory.NewAdminUserWith(op)),
				Checker:         authz.NewChecker(startPolicyCerbos(t)),
			}
			e := echo.New()
			e.HTTPErrorHandler = presentation.CustomHTTPErrorHandler
			e.Validator = testValidator{v: validator.New()}
			presentation.RegisterRoutes(e, h)

			tok, err := sess.Issue(op.ID(), time.Now())
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+w.ID().String()+"/transfer-owner",
				strings.NewReader(`{"newOwnerUserId":"`+member.ID().String()+`"}`))
			req.Header.Set("Content-Type", "application/json")
			req.AddCookie(&http.Cookie{Name: session.CookieName, Value: tok})
			req.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: csrfToken})
			req.Header.Set(session.CSRFHeaderName, csrfToken)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			require.Equal(t, tt.status, rec.Code, rec.Body.String())
			if tt.wantMsg != "" {
				var body internal.ErrorResponse
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
				assert.Equal(t, tt.wantMsg, body.Message)
			}
			saved, err := wsRepo.FindByID(t.Context(), w.ID())
			require.NoError(t, err)
			want := role.RoleOwner
			if tt.status == http.StatusOK {
				want = role.RoleMaintainer
			}
			assert.Equal(t, want, saved.Members().UserRole(owner.ID()))
		})
	}
}
