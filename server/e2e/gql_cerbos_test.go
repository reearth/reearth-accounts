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

// baseSeederOneUserWithSelfRoleNoPermittable creates user, workspace, and "self" role WITHOUT creating permittable
// This is used for testing permission checks when permittable doesn't exist yet
func baseSeederOneUserWithSelfRoleNoPermittable(ctx context.Context, r *repo.Container) error {
	if err := seedRoles(ctx, r); err != nil {
		return err
	}

	auth := user.ReearthSub(uId.String())
	metadata := user.NewMetadata()
	metadata.LangFrom("ja")
	metadata.SetTheme(user.ThemeDark)

	u := user.New().ID(uId).
		Name("e2e").
		Email("e2e@e2e.com").
		Auths([]user.Auth{*auth}).
		Metadata(metadata).
		Workspace(wId).
		MustBuild()
	if err := r.User.Save(ctx, u); err != nil {
		return err
	}
	roleOwner := workspace.Member{
		Role:      role.RoleOwner,
		InvitedBy: uId,
	}

	w := workspace.New().ID(wId).
		Name("e2e").
		Members(map[idx.ID[id.User]]workspace.Member{
			uId: roleOwner,
		}).
		Integrations(map[idx.ID[id.Integration]]workspace.Member{
			iId: roleOwner,
		}).
		MustBuild()
	if err := r.Workspace.Save(ctx, w); err != nil {
		return err
	}

	return nil
}

// baseSeederOneUserWithPermittable creates user, workspace, roles, and a permittable with "role1"
func baseSeederOneUserWithPermittable(ctx context.Context, r *repo.Container) error {
	if err := baseSeederOneUserWithSelfRoleNoPermittable(ctx, r); err != nil {
		return err
	}

	selfRole, err := r.Role.FindByName(ctx, role.RoleSelf.String())
	if err != nil {
		return err
	}

	cerbosRole := role.New().NewID().Name("role1").MustBuild()
	if err := r.Role.Save(ctx, *cerbosRole); err != nil {
		return err
	}

	p, err := permittable.New().
		NewID().
		UserID(uId).
		RoleIDs([]id.RoleID{selfRole.ID(), cerbosRole.ID()}).
		Build()
	if err != nil {
		return err
	}
	return r.Permittable.Save(ctx, *p)
}

func checkPermission(e *httpexpect.Expect, service string, resource string, action string) (GraphQLRequest, *httpexpect.Value) {
	checkPermissionRequestBody := GraphQLRequest{
		OperationName: "CheckPermission",
		Query: `query CheckPermission($input: CheckPermissionInput!) {
			checkPermission(input: $input) {
				allowed
			}
		}`,
		Variables: map[string]any{
			"input": map[string]any{
				"service":  service,
				"resource": resource,
				"action":   action,
			},
		},
	}

	res := e.POST("/api/graphql").
		WithHeader("Origin", "https://example.com").
		WithHeader("authorization", "Bearer test").
		WithHeader("X-Reearth-Debug-User", uId.String()).
		WithHeader("Content-Type", "application/json").
		WithJSON(checkPermissionRequestBody).
		Expect().
		Status(http.StatusOK).
		JSON()

	return checkPermissionRequestBody, res
}

func TestCheckPermission(t *testing.T) {
	container, err := newCerbosContainer()
	if err != nil {
		t.Fatalf("failed to start cerbos container: %v", err)
	}
	defer func() {
		if err = container.terminate(); err != nil {
			t.Errorf("failed to terminate container: %v", err)
		}
	}()

	t.Run("should deny when no permittable exists", func(t *testing.T) {
		e, _ := StartServer(t, &app.Config{
			CerbosHost: container.getAddress(),
		}, true, baseSeederOneUserWithSelfRoleNoPermittable)

		_, res := checkPermission(e, "service", "resource", "read")
		res.Object().
			Value("data").Object().
			Value("checkPermission").Object().
			Value("allowed").Boolean().IsFalse()
	})

	t.Run("should allow when permittable with role exists", func(t *testing.T) {
		e, _ := StartServer(t, &app.Config{
			CerbosHost: container.getAddress(),
		}, true, baseSeederOneUserWithPermittable)

		_, res := checkPermission(e, "service", "resource", "read")
		res.Object().
			Value("data").Object().
			Value("checkPermission").Object().
			Value("allowed").Boolean().IsTrue()
	})

	t.Run("should deny for unauthorized action", func(t *testing.T) {
		e, _ := StartServer(t, &app.Config{
			CerbosHost: container.getAddress(),
		}, true, baseSeederOneUserWithPermittable)

		_, res := checkPermission(e, "service", "resource", "edit")
		res.Object().
			Value("data").Object().
			Value("checkPermission").Object().
			Value("allowed").Boolean().IsFalse()
	})
}
