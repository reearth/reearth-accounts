package e2e

import (
	"context"
	"net/http"
	"testing"

	httpexpect "github.com/gavv/httpexpect/v2"
	"github.com/reearth/reearth-accounts/server/internal/app"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/idx"
)

// baseSeederOneUserNoPermittable creates a user and workspace WITHOUT creating permittable
// This is used for testing permittable CRUD operations from scratch
func baseSeederOneUserNoPermittable(ctx context.Context, r *repo.Container) error {
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

	// NOTE: Intentionally NOT creating permittable - this test wants to test creating it via GraphQL
	return nil
}

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
		WithHeader("X-Reearth-Debug-User", uId.String()).
		WithHeader("Content-Type", "application/json").
		WithJSON(getUsersWithRolesRequestBody).
		Expect().
		Status(http.StatusOK).
		JSON()

	return getUsersWithRolesRequestBody, res
}

func TestGetUsersWithRoles(t *testing.T) {
	e, _ := StartServer(t, &app.Config{}, true, baseSeederOneUserNoPermittable)

	_, res := getUsersWithRoles(e)
	res.Object().
		Value("data").Object().
		Value("getUsersWithRoles").Object().
		Value("usersWithRoles").Array().NotEmpty()
	res.Object().
		Value("data").Object().
		Value("getUsersWithRoles").Object().
		Value("usersWithRoles").Array().Length().IsEqual(1)
	res.Object().
		Value("data").Object().
		Value("getUsersWithRoles").Object().
		Value("usersWithRoles").Array().Value(0).Object().
		Value("user").Object().
		Value("id").String().IsEqual(uId.String())
	res.Object().
		Value("data").Object().
		Value("getUsersWithRoles").Object().
		Value("usersWithRoles").Array().Value(0).Object().
		Value("roles").Array().IsEmpty()
}
