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

// baseSeederUserNoRoles creates users and workspaces WITHOUT creating roles
// This is used for testing role CRUD operations from scratch
func baseSeederUserNoRoles(ctx context.Context, r *repo.Container) error {
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
	u2 := user.New().ID(uId2).
		Name("e2e2").
		Workspace(wId2).
		Metadata(metadata).
		Email("e2e2@e2e.com").
		MustBuild()
	if err := r.User.Save(ctx, u2); err != nil {
		return err
	}
	u3 := user.New().ID(uId3).
		Name("e2e3").
		Workspace(wId2).
		Metadata(metadata).
		Email("e2e3@e2e.com").
		MustBuild()
	if err := r.User.Save(ctx, u3); err != nil {
		return err
	}
	roleOwner := workspace.Member{
		Role:      workspace.RoleOwner,
		InvitedBy: uId,
	}
	roleReader := workspace.Member{
		Role:      workspace.RoleReader,
		InvitedBy: uId2,
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

	w2 := workspace.New().ID(wId2).
		Name("e2e2").
		Members(map[idx.ID[id.User]]workspace.Member{
			uId:  roleOwner,
			uId3: roleReader,
		}).
		Integrations(map[idx.ID[id.Integration]]workspace.Member{
			iId: roleOwner,
		}).
		MustBuild()
	if err := r.Workspace.Save(ctx, w2); err != nil {
		return err
	}

	// NOTE: Intentionally NOT creating roles - this test wants to test role CRUD from scratch
	// NOTE: Also NOT creating permittables since roles don't exist yet
	return nil
}

func getRoles(e *httpexpect.Expect) (GraphQLRequest, *httpexpect.Value) {
	getRolesRequestBody := GraphQLRequest{
		OperationName: "GetRoles",
		Query: `query GetRoles {
			roles {
				roles {
					id
					name
				}
			}
		}`,
		Variables: map[string]any{},
	}

	res := e.POST("/api/graphql").
		WithHeader("Origin", "https://example.com").
		WithHeader("authorization", "Bearer test").
		WithHeader("X-Reearth-Debug-User", uID.String()).
		WithHeader("Content-Type", "application/json").
		WithJSON(getRolesRequestBody).
		Expect().
		Status(http.StatusOK).
		JSON()

	return getRolesRequestBody, res
}

func addRole(e *httpexpect.Expect, roleName string) (GraphQLRequest, *httpexpect.Value, string) {
	addRoleRequestBody := GraphQLRequest{
		OperationName: "AddRole",
		Query: `mutation AddRole($input: AddRoleInput!) {
			addRole(input: $input) {
				role {
					id
					name
				}
			}
		}`,
		Variables: map[string]any{
			"input": map[string]any{
				"name": roleName,
			},
		},
	}

	res := e.POST("/api/graphql").
		WithHeader("Origin", "https://example.com").
		WithHeader("authorization", "Bearer test").
		WithHeader("X-Reearth-Debug-User", uID.String()).
		WithHeader("Content-Type", "application/json").
		WithJSON(addRoleRequestBody).
		Expect().
		Status(http.StatusOK).
		JSON()

	roleId := res.Path("$.data.addRole.role.id").String().Raw()
	return addRoleRequestBody, res, roleId
}

func updateRole(e *httpexpect.Expect, roleID string, newName string) (GraphQLRequest, *httpexpect.Value, string) {
	updateRoleRequestBody := GraphQLRequest{
		OperationName: "UpdateRole",
		Query: `mutation UpdateRole($input: UpdateRoleInput!) {
			updateRole(input: $input) {
				role {
					id
					name
				}
			}
		}`,
		Variables: map[string]any{
			"input": map[string]any{
				"id":   roleID,
				"name": newName,
			},
		},
	}

	res := e.POST("/api/graphql").
		WithHeader("Origin", "https://example.com").
		WithHeader("authorization", "Bearer test").
		WithHeader("X-Reearth-Debug-User", uID.String()).
		WithHeader("Content-Type", "application/json").
		WithJSON(updateRoleRequestBody).
		Expect().
		Status(http.StatusOK).
		JSON()

	roleId := res.Path("$.data.updateRole.role.id").String().Raw()
	return updateRoleRequestBody, res, roleId
}

func removeRole(e *httpexpect.Expect, roleID string) (GraphQLRequest, *httpexpect.Value, string) {
	removeRoleRequestBody := GraphQLRequest{
		OperationName: "RemoveRole",
		Query: `mutation RemoveRole($input: RemoveRoleInput!) {
			removeRole(input: $input) {
				id
			}
		}`,
		Variables: map[string]any{
			"input": map[string]any{
				"id": roleID,
			},
		},
	}

	res := e.POST("/api/graphql").
		WithHeader("Origin", "https://example.com").
		WithHeader("authorization", "Bearer test").
		WithHeader("X-Reearth-Debug-User", uID.String()).
		WithHeader("Content-Type", "application/json").
		WithJSON(removeRoleRequestBody).
		Expect().
		Status(http.StatusOK).
		JSON()

	roleId := res.Path("$.data.removeRole.id").String().Raw()

	return removeRoleRequestBody, res, roleId
}

func TestRoleCRUD(t *testing.T) {
	e, _ := StartServer(t, &app.Config{}, true, baseSeederUserNoRoles)

	// Get roles and check if there are no roles
	_, res1 := getRoles(e)
	res1.Object().
		Value("data").Object().
		Value("roles").Object().
		Value("roles").Array().IsEmpty()

	// Add a new role
	_, _, roleId1 := addRole(e, "TestRole")

	// Get roles and check if the new role is present
	_, res3 := getRoles(e)
	res3.Object().
		Value("data").Object().
		Value("roles").Object().
		Value("roles").Array().Length().IsEqual(1)
	res3.Object().
		Value("data").Object().
		Value("roles").Object().
		Value("roles").Array().Value(0).Object().
		Value("id").String().IsEqual(roleId1)
	res3.Object().
		Value("data").Object().
		Value("roles").Object().
		Value("roles").Array().Value(0).Object().
		Value("name").String().IsEqual("TestRole")

	// update the role
	_, _, _ = updateRole(e, roleId1, "UpdatedTestRole")

	// Get roles and check if the role has been updated
	_, res4 := getRoles(e)
	res4.Object().
		Value("data").Object().
		Value("roles").Object().
		Value("roles").Array().Length().IsEqual(1)
	res4.Object().
		Value("data").Object().
		Value("roles").Object().
		Value("roles").Array().Value(0).Object().
		Value("id").String().IsEqual(roleId1)
	res4.Object().
		Value("data").Object().
		Value("roles").Object().
		Value("roles").Array().Value(0).Object().
		Value("name").String().IsEqual("UpdatedTestRole")

	// Remove the role
	_, _, _ = removeRole(e, roleId1)

	// Get roles and check if there are no roles
	_, res5 := getRoles(e)
	res5.Object().
		Value("data").Object().
		Value("roles").Object().
		Value("roles").Array().IsEmpty()
}
