package presentation_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
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
	"github.com/reearth/reearth-accounts/server/internal/admin/auth/session"
	adminpresentation "github.com/reearth/reearth-accounts/server/internal/admin/presentation"
	workspacehandler "github.com/reearth/reearth-accounts/server/internal/admin/presentation/handler/workspace"
	mw "github.com/reearth/reearth-accounts/server/internal/admin/presentation/middleware"
	adminrbac "github.com/reearth/reearth-accounts/server/internal/admin/rbac"
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

type structValidator struct{ v *validator.Validate }

func (s structValidator) Validate(i any) error {
	if err := s.v.Struct(i); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return nil
}

func TestRoutes_TransferWorkspaceOwner_ByRole(t *testing.T) {
	tests := []struct {
		name   string
		role   adminuser.Role
		status int
	}{
		{name: "viewer is forbidden", role: adminuser.RoleViewer, status: http.StatusForbidden},
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
			sess := session.NewManager("test-secret-test-secret-test-secret", time.Hour)

			h := &adminpresentation.Handler{
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
			e.HTTPErrorHandler = adminpresentation.CustomHTTPErrorHandler
			e.Validator = structValidator{v: validator.New()}
			adminpresentation.RegisterRoutes(e, h)

			tok, err := sess.Issue(op.ID(), time.Now())
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+w.ID().String()+"/transfer-owner",
				strings.NewReader(`{"newOwnerUserId":"`+member.ID().String()+`"}`))
			req.Header.Set("Content-Type", "application/json")
			req.AddCookie(&http.Cookie{Name: session.CookieName, Value: tok})
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			assert.Equal(t, tt.status, rec.Code, rec.Body.String())
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
