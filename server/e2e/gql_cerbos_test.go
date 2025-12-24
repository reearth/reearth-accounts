package e2e

import (
	"context"
	"net/http"
	"testing"

	httpexpect "github.com/gavv/httpexpect/v2"
	"github.com/reearth/reearth-accounts/server/internal/app"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/idx"
)

// baseSeederOneUserWithSelfRoleNoPermittable creates user, workspace, and "self" role WITHOUT creating permittable
// This is used for testing permission checks when permittable doesn't exist yet
func baseSeederOneUserWithSelfRoleNoPermittable(ctx context.Context, r *repo.Container) error {
	// First seed the self role (required for CheckPermission to work)
	if err := seedRoles(ctx, r); err != nil {
		return err
	}

	auth := user.ReearthSub(uID.String())
	metadata := user.NewMetadata()
	metadata.LangFrom("ja")
	metadata.SetTheme(user.ThemeDark)

	u := user.New().ID(uID).
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
		Role:      workspace.RoleOwner,
		InvitedBy: uID,
	}

	w := workspace.New().ID(wId).
		Name("e2e").
		Members(map[idx.ID[id.User]]workspace.Member{
			uID: roleOwner,
		}).
		Integrations(map[idx.ID[id.Integration]]workspace.Member{
			iId: roleOwner,
		}).
		MustBuild()
	if err := r.Workspace.Save(ctx, w); err != nil {
		return err
	}

	// NOTE: Intentionally NOT creating permittable - this test wants to test permission denied scenario
	return nil
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
		WithHeader("X-Reearth-Debug-User", uID.String()).
		WithHeader("Content-Type", "application/json").
		WithJSON(checkPermissionRequestBody).
		Expect().
		Status(http.StatusOK).
		JSON()

	return checkPermissionRequestBody, res
}

func TestCheckPermission(t *testing.T) {
	// Start Cerbos container
	container, err := newCerbosContainer()
	if err != nil {
		t.Fatalf("failed to start cerbos container: %v", err)
	}
	defer func() {
		if err = container.terminate(); err != nil {
			t.Errorf("failed to terminate container: %v", err)
		}
	}()

	// Start server with user and roles but no permittable
	e, _ := StartServer(t, &app.Config{
		CerbosHost: container.getAddress(),
	}, true, baseSeederOneUserWithSelfRoleNoPermittable)

	// check permission with no permittable - should be denied (user has no roles in permittable)
	_, res1 := checkPermission(e, "service", "resource", "read")
	res1.Object().
		Value("data").Object().
		Value("checkPermission").Object().
		Value("allowed").Boolean().IsFalse()

	// Add role and permittable
	_, _, roleId1 := addRole(e, "role1")
	_, _, _ = updatePermittable(e, uID.String(), []string{roleId1})

	// check permission with permittable
	_, res2 := checkPermission(e, "service", "resource", "read")
	res2.Object().
		Value("data").Object().
		Value("checkPermission").Object().
		Value("allowed").Boolean().IsTrue()

	// check permission with permittable but allowed is false
	_, res3 := checkPermission(e, "service", "resource", "edit")
	res3.Object().
		Value("data").Object().
		Value("checkPermission").Object().
		Value("allowed").Boolean().IsFalse()
}
