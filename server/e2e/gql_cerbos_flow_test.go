package e2e

import (
	"context"
	"net/http"
	"testing"

	httpexpect "github.com/gavv/httpexpect/v2"
	"github.com/reearth/reearth-accounts/server/internal/app"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/idx"
)

const flowWsAlias = "flow-e2e-ws"

// seederFlowWorkspaceOwner seeds a user who is OWNER of a workspace via
// permittable.workspace_roles, while the global roleids contain only "self" —
// i.e. the exact production shape after the FixPermittableRoleIDs migration.
func seederFlowWorkspaceOwner(ctx context.Context, r *repo.Container) error {
	if err := seedRoles(ctx, r); err != nil {
		return err
	}
	selfRole, err := r.Role.FindByName(ctx, role.RoleSelf.String())
	if err != nil {
		return err
	}
	ownerRole, err := r.Role.FindByName(ctx, role.RoleOwner.String())
	if err != nil {
		return err
	}

	metadata := user.NewMetadata()
	metadata.LangFrom("ja")
	metadata.SetTheme(user.ThemeDark)
	u := user.New().ID(uId).
		Name("flow-e2e").
		Email("flow-e2e@e2e.com").
		Auths([]user.Auth{*user.ReearthSub(uId.String())}).
		Metadata(metadata).
		Workspace(wId).
		MustBuild()
	if err := r.User.Save(ctx, u); err != nil {
		return err
	}

	w := workspace.New().ID(wId).
		Name("flow-e2e").
		Alias(flowWsAlias).
		Members(map[idx.ID[id.User]]workspace.Member{
			uId: {Role: role.RoleOwner, InvitedBy: uId},
		}).
		Integrations(map[idx.ID[id.Integration]]workspace.Member{
			iId: {Role: role.RoleOwner, InvitedBy: uId},
		}).
		MustBuild()
	if err := r.Workspace.Save(ctx, w); err != nil {
		return err
	}

	// global roleids = [self] only; the owner authority lives in workspace_roles,
	// keyed by the workspace.
	p, err := permittable.New().
		NewID().
		UserID(uId).
		RoleIDs([]id.RoleID{selfRole.ID()}).
		WorkspaceRoles([]permittable.WorkspaceRole{
			permittable.NewWorkspaceRole(wId, ownerRole.ID()),
		}).
		Build()
	if err != nil {
		return err
	}
	return r.Permittable.Save(ctx, *p)
}

func checkPermissionWithAlias(e *httpexpect.Expect, service, resource, action string, workspaceAlias *string) *httpexpect.Value {
	input := map[string]any{"service": service, "resource": resource, "action": action}
	if workspaceAlias != nil {
		input["workspaceAlias"] = *workspaceAlias
	}
	body := GraphQLRequest{
		OperationName: "CheckPermission",
		Query: `query CheckPermission($input: CheckPermissionInput!) {
			checkPermission(input: $input) {
				allowed
			}
		}`,
		Variables: map[string]any{"input": input},
	}
	return e.POST("/api/graphql").
		WithHeader("Origin", "https://example.com").
		WithHeader("authorization", "Bearer test").
		WithHeader("X-Reearth-Debug-User", uId.String()).
		WithHeader("Content-Type", "application/json").
		WithJSON(body).
		Expect().
		Status(http.StatusOK).
		JSON()
}

// TestCheckPermission_FlowWorkspaceOwner validates that workspace-specific roles
// are resolved into the Cerbos principal only when a workspace alias is supplied.
//
// A workspace OWNER whose global roleids are "self"-only is allowed the
// owner-gated flow:deployment action ONLY when the request carries the workspace
// alias (so checkWorkspacePermission resolves workspace_roles); without the alias
// the principal is global-self only and is denied. This mirrors the Re:Earth Flow
// workspace-scoped permission fix end to end (accounts server + Cerbos).
func TestCheckPermission_FlowWorkspaceOwner(t *testing.T) {
	container, err := newCerbosContainer()
	if err != nil {
		t.Fatalf("failed to start cerbos container: %v", err)
	}
	defer func() {
		if err = container.terminate(); err != nil {
			t.Errorf("failed to terminate container: %v", err)
		}
	}()

	alias := flowWsAlias

	t.Run("owner with workspace alias is allowed", func(t *testing.T) {
		e, _ := StartServer(t, &app.Config{CerbosHost: container.getAddress()}, true, seederFlowWorkspaceOwner)
		checkPermissionWithAlias(e, "flow", "deployment", "any", &alias).
			Object().Value("data").Object().
			Value("checkPermission").Object().
			Value("allowed").Boolean().IsTrue()
	})

	t.Run("owner without alias (self-only global) is denied", func(t *testing.T) {
		e, _ := StartServer(t, &app.Config{CerbosHost: container.getAddress()}, true, seederFlowWorkspaceOwner)
		checkPermissionWithAlias(e, "flow", "deployment", "any", nil).
			Object().Value("data").Object().
			Value("checkPermission").Object().
			Value("allowed").Boolean().IsFalse()
	})
}
