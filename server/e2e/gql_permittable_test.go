package e2e

import (
	"net/http"
	"testing"

	httpexpect "github.com/gavv/httpexpect/v2"
	"github.com/reearth/reearth-accounts/server/internal/app"
)

func getUsersWithRoles(e *httpexpect.Expect) (GraphQLRequest, *httpexpect.Value) {
	getUsersWithRolesRequestBody := GraphQLRequest{
		OperationName: "GetUsersWithRoles",
		Query: `query GetUsersWithRoles {
			getUsersWithRoles {
				usersWithRoles {
					user {
						id
						name
						email
					}
					roles {
						id
						name
					}
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
		WithJSON(getUsersWithRolesRequestBody).
		Expect().
		Status(http.StatusOK).
		JSON()

	return getUsersWithRolesRequestBody, res
}

func updatePermittable(e *httpexpect.Expect, userID string, roleIDs []string) (GraphQLRequest, *httpexpect.Value, string) {
	updatePermittableRequestBody := GraphQLRequest{
		OperationName: "UpdatePermittable",
		Query: `mutation UpdatePermittable($input: UpdatePermittableInput!) {
			updatePermittable(input: $input) {
				permittable {
					id
					userId
					roleIds
				}
			}
		}`,
		Variables: map[string]any{
			"input": map[string]any{
				"userId":  userID,
				"roleIds": roleIDs,
			},
		},
	}

	res := e.POST("/api/graphql").
		WithHeader("Origin", "https://example.com").
		WithHeader("authorization", "Bearer test").
		WithHeader("X-Reearth-Debug-User", uID.String()).
		WithHeader("Content-Type", "application/json").
		WithJSON(updatePermittableRequestBody).
		Expect().
		Status(http.StatusOK).
		JSON()

	permittableId := res.Path("$.data.updatePermittable.permittable.id").String().Raw()
	return updatePermittableRequestBody, res, permittableId
}

func TestPermittableCRUD(t *testing.T) {
	e, _ := StartServer(t, &app.Config{}, true, nil)

	// Get users and roles check if users are empty
	_, res1 := getUsersWithRoles(e)
	res1.Object().
		Value("data").Object().
		Value("getUsersWithRoles").Object().
		Value("usersWithRoles").Array().IsEmpty()

	e, _ = StartServer(t, &app.Config{}, true, baseSeederOneUser)

	// Get users and roles check if users are not empty and roles are empty
	_, res2 := getUsersWithRoles(e)
	res2.Object().
		Value("data").Object().
		Value("getUsersWithRoles").Object().
		Value("usersWithRoles").Array().NotEmpty()
	res2.Object().
		Value("data").Object().
		Value("getUsersWithRoles").Object().
		Value("usersWithRoles").Array().Length().IsEqual(1)
	res2.Object().
		Value("data").Object().
		Value("getUsersWithRoles").Object().
		Value("usersWithRoles").Array().Value(0).Object().
		Value("user").Object().
		Value("id").String().IsEqual(uId.String())
	res2.Object().
		Value("data").Object().
		Value("getUsersWithRoles").Object().
		Value("usersWithRoles").Array().Value(0).Object().
		Value("roles").Array().IsEmpty()

	// Add role to user
	_, _, roleId1 := addRole(e, "TestRole1")
	_, _, _ = updatePermittable(e, uId.String(), []string{roleId1})

	// Get users and roles check if the role is present
	_, res3 := getUsersWithRoles(e)
	res3.Object().
		Value("data").Object().
		Value("getUsersWithRoles").Object().
		Value("usersWithRoles").Array().Value(0).Object().
		Value("roles").Array().Length().IsEqual(1)
	res3.Object().
		Value("data").Object().
		Value("getUsersWithRoles").Object().
		Value("usersWithRoles").Array().Value(0).Object().
		Value("roles").Array().Value(0).Object().
		Value("id").String().IsEqual(roleId1)
	res3.Object().
		Value("data").Object().
		Value("getUsersWithRoles").Object().
		Value("usersWithRoles").Array().Value(0).Object().
		Value("roles").Array().Value(0).Object().
		Value("name").String().IsEqual("TestRole1")

	// Update role to user
	_, _, roleId2 := addRole(e, "TestRole2")
	_, _, roleId3 := addRole(e, "TestRole3")
	_, _, _ = updatePermittable(e, uId.String(), []string{roleId2, roleId3})

	// Get users and roles check if the roles are present
	_, res4 := getUsersWithRoles(e)
	res4.Object().
		Value("data").Object().
		Value("getUsersWithRoles").Object().
		Value("usersWithRoles").Array().Value(0).Object().
		Value("roles").Array().Length().IsEqual(2)
	res4.Object().
		Value("data").Object().
		Value("getUsersWithRoles").Object().
		Value("usersWithRoles").Array().Value(0).Object().
		Value("roles").Array().ContainsAll(
		map[string]interface{}{
			"id":   roleId2,
			"name": "TestRole2",
		},
		map[string]interface{}{
			"id":   roleId3,
			"name": "TestRole3",
		},
	)
}
