package e2e

import (
	"context"
	"encoding/json"
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
	"github.com/stretchr/testify/require"
)

const (
	victimEmail = "victim@example.com"
	viewerEmail = "viewer@example.com"
)

// The viewer shares a workspace with the victim, so these tests also pin that
// workspace co-members do not see each other's email.
func seedEmailVisibility(ctx context.Context, r *repo.Container) error {
	for _, u := range []*user.User{
		user.New().ID(uId).Name("victim").Alias("victim").Email(victimEmail).
			Auths([]user.Auth{*user.ReearthSub(uId.String())}).Workspace(wId).MustBuild(),
		user.New().ID(uId2).Name("viewer").Alias("viewer").Email(viewerEmail).
			Auths([]user.Auth{*user.ReearthSub(uId2.String())}).Workspace(wId2).MustBuild(),
	} {
		if err := r.User.Save(ctx, u); err != nil {
			return err
		}
	}
	return r.Workspace.Save(ctx, workspace.New().ID(wId).Name("team").Alias("team-alias").
		Members(map[idx.ID[id.User]]workspace.Member{
			uId:  {Role: role.RoleOwner, InvitedBy: uId},
			uId2: {Role: role.RoleReader, InvitedBy: uId},
		}).MustBuild())
}

func postGQL(t *testing.T, e *httpexpect.Expect, viewer *id.UserID, query string) *httpexpect.Object {
	t.Helper()
	b, err := json.Marshal(GraphQLRequest{Query: query})
	require.NoError(t, err)
	req := e.POST("/api/graphql").WithHeader("Content-Type", "application/json").WithBytes(b)
	if viewer != nil {
		req = req.WithHeader("authorization", "Bearer test").WithHeader("X-Reearth-Debug-User", viewer.String())
	}
	o := req.Expect().Status(http.StatusOK).JSON().Object()
	o.NotContainsKey("errors")
	return o.Value("data").Object()
}

var anonymousEmailQueries = map[string]string{
	"findUsersByIDsWithPagination": `{ r: findUsersByIDsWithPagination(ids:["` + uId.String() + `"], pagination:{page:1,size:10}) { users { email } } }`,
	"findByID":                     `{ r: findByID(id:"` + wId.String() + `") { members { ... on WorkspaceUserMember { user { email } } } } }`,
	"findByIDs":                    `{ r: findByIDs(ids:["` + wId.String() + `"]) { members { ... on WorkspaceUserMember { user { email } } } } }`,
	"findByAlias":                  `{ r: findByAlias(alias:"team-alias") { members { ... on WorkspaceUserMember { user { email } } } } }`,
}

func TestUserEmail_HiddenFromAnonymousCallers(t *testing.T) {
	e, _ := StartServerNoDebug(t, &app.Config{}, false, seedEmailVisibility)
	for name, q := range anonymousEmailQueries {
		t.Run(name, func(t *testing.T) {
			raw := postGQL(t, e, nil, q).Raw()
			b, _ := json.Marshal(raw)
			require.NotContains(t, string(b), victimEmail)
			require.Contains(t, string(b), `"email":""`)
		})
	}
}

func TestUserEmail_HiddenFromOtherSignedInUsers(t *testing.T) {
	e, _ := StartServer(t, &app.Config{}, false, seedEmailVisibility)
	viewer := uId2

	for name, q := range map[string]string{
		"findUsersByIDs":    `{ r: findUsersByIDs(ids:["` + uId.String() + `"]) { email } }`,
		"user":              `{ r: user(id:"` + uId.String() + `") { email } }`,
		"findUserByAlias":   `{ r: findUserByAlias(alias:"victim") { email } }`,
		"userByNameOrAlias": `{ r: userByNameOrAlias(nameOrAlias:"victim") { email } }`,
		"userByNameOrEmail": `{ r: userByNameOrEmail(nameOrEmail:"` + victimEmail + `") { email } }`,
		"searchUserByName":  `{ r: searchUser(keyword:"victim") { email } }`,
		"workspaceMembers":  `{ r: findByID(id:"` + wId.String() + `") { members { ... on WorkspaceUserMember { user { email } } } } }`,
	} {
		t.Run(name, func(t *testing.T) {
			raw := postGQL(t, e, &viewer, q).Raw()
			b, _ := json.Marshal(raw)
			require.NotContains(t, string(b), victimEmail)
			require.Contains(t, string(b), `"email":""`)
		})
	}
}

func TestSearchUser_MatchesWholeEmailOnly(t *testing.T) {
	e, _ := StartServer(t, &app.Config{}, false, seedEmailVisibility)
	viewer := uId2

	postGQL(t, e, &viewer, `{ searchUser(keyword:"example.com") { id } }`).
		Value("searchUser").Array().IsEmpty()
	postGQL(t, e, &viewer, `{ searchUser(keyword:"victim@exam") { id } }`).
		Value("searchUser").Array().IsEmpty()

	hits := postGQL(t, e, &viewer, `{ searchUser(keyword:"VICTIM@example.com") { id name email } }`).
		Value("searchUser").Array()
	hits.Length().IsEqual(1)
	hits.Value(0).Object().Value("id").String().IsEqual(uId.String())
	hits.Value(0).Object().Value("email").String().IsEqual("")
}

func TestUserEmail_VisibleToOwner(t *testing.T) {
	e, _ := StartServer(t, &app.Config{}, false, seedEmailVisibility)
	viewer := uId2

	postGQL(t, e, &viewer, `{ me { email } }`).
		Value("me").Object().Value("email").String().IsEqual(viewerEmail)
	postGQL(t, e, &viewer, `{ findUsersByIDs(ids:["`+uId2.String()+`","`+uId.String()+`"]) { id email } }`).
		Value("findUsersByIDs").Array().Filter(func(_ int, v *httpexpect.Value) bool {
		return v.Object().Value("id").String().Raw() == uId2.String()
	}).Value(0).Object().Value("email").String().IsEqual(viewerEmail)
}

func TestUserEmail_SignupReturnsOwnEmail(t *testing.T) {
	e, _ := StartServerNoDebug(t, &app.Config{}, false, seedRoles)
	b, err := json.Marshal(GraphQLRequest{
		Query: `mutation($input: SignupInput!) { signup(input: $input) { user { email } } }`,
		Variables: map[string]any{"input": map[string]any{
			"email": "newuser@example.com", "name": "new user", "password": "StrongPassw0rd!",
		}},
	})
	require.NoError(t, err)
	e.POST("/api/graphql").WithHeader("Content-Type", "application/json").WithBytes(b).
		Expect().Status(http.StatusOK).JSON().Object().
		Value("data").Object().Value("signup").Object().Value("user").Object().
		Value("email").String().IsEqual("newuser@example.com")
}
