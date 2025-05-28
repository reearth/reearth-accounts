package e2e

import (
	"net/http"
	"testing"

	httpexpect "github.com/gavv/httpexpect/v2"
	"github.com/reearth/reearth-accounts/server/internal/app"
)

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
	e, _ := StartServer(t, &app.Config{}, true, baseSeederUser)

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
