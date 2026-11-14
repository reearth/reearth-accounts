package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/gavv/httpexpect/v2"
	"github.com/reearth/reearth-accounts/server/internal/app"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type photoUsers struct {
	viewer, withPhoto, noPhoto id.UserID
	ws                         id.WorkspaceID
}

func seedPhotoUsers(f photoUsers) Seeder {
	return func(ctx context.Context, r *repo.Container) error {
		save := func(uid id.UserID, name, photo string) error {
			md := user.MetadataFrom(photo, "", "", user.NewMetadata().Lang(), user.ThemeDefault)
			return r.User.Save(ctx, user.New().ID(uid).Name(name).Email(name+"@example.com").
				Workspace(f.ws).Metadata(md).MustBuild())
		}
		if err := save(f.viewer, "photoviewer", "users/viewer.webp"); err != nil {
			return err
		}
		if err := save(f.withPhoto, "photoowner", "users/owner.webp"); err != nil {
			return err
		}
		if err := save(f.noPhoto, "photoless", ""); err != nil {
			return err
		}
		members := map[id.UserID]workspace.Member{
			f.viewer:    {Role: role.RoleOwner, InvitedBy: f.viewer},
			f.withPhoto: {Role: role.RoleReader, InvitedBy: f.viewer},
			f.noPhoto:   {Role: role.RoleReader, InvitedBy: f.viewer},
		}
		return r.Workspace.Save(ctx, workspace.New().ID(f.ws).Name("photos").Alias("photos").Members(members).MustBuild())
	}
}

func startPhotoServer(t *testing.T) (*httpexpect.Expect, photoUsers) {
	f := photoUsers{viewer: id.NewUserID(), withPhoto: id.NewUserID(), noPhoto: id.NewUserID(), ws: id.NewWorkspaceID()}
	e, _ := StartServer(t, &app.Config{StorageIsLocal: true}, false, seedPhotoUsers(f))
	return e, f
}

func photoQuery(t *testing.T, e *httpexpect.Expect, viewer id.UserID, query string) map[string]any {
	t.Helper()
	body, err := json.Marshal(GraphQLRequest{Query: query})
	require.NoError(t, err)
	raw := e.POST("/api/graphql").
		WithHeader("authorization", "Bearer test").
		WithHeader("Content-Type", "application/json").
		WithHeader("X-Reearth-Debug-User", viewer.String()).
		WithBytes(body).
		Expect().Status(http.StatusOK).Body().Raw()
	var res struct {
		Data   map[string]any `json:"data"`
		Errors []any          `json:"errors"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &res))
	require.Empty(t, res.Errors)
	return res.Data
}

func photoOf(u any) string {
	return u.(map[string]any)["metadata"].(map[string]any)["photoURL"].(string)
}

func assertSigned(t *testing.T, got, stored string) {
	t.Helper()
	assert.True(t, strings.HasPrefix(got, "https://"), "want a signed URL, got %q", got)
	assert.Contains(t, got, stored)
	assert.Contains(t, got, "Signature=")
}

func TestGQL_FindUsersByIDsSignsPhotoURL(t *testing.T) {
	e, f := startPhotoServer(t)
	data := photoQuery(t, e, f.viewer, fmt.Sprintf(
		`{ findUsersByIDs(ids: ["%s", "%s"]) { id metadata { photoURL } } }`, f.withPhoto, f.noPhoto))

	got := map[string]string{}
	for _, u := range data["findUsersByIDs"].([]any) {
		got[u.(map[string]any)["id"].(string)] = photoOf(u)
	}
	require.Len(t, got, 2)
	assertSigned(t, got[f.withPhoto.String()], "users/owner.webp")
	assert.Empty(t, got[f.noPhoto.String()])
}

func TestGQL_SearchUserSignsPhotoURL(t *testing.T) {
	e, f := startPhotoServer(t)
	data := photoQuery(t, e, f.viewer, `{ searchUser(keyword: "photoowner") { id metadata { photoURL } } }`)

	users := data["searchUser"].([]any)
	require.Len(t, users, 1)
	assertSigned(t, photoOf(users[0]), "users/owner.webp")
}

func TestGQL_WorkspaceMemberUserSignsPhotoURL(t *testing.T) {
	e, f := startPhotoServer(t)
	data := photoQuery(t, e, f.viewer, fmt.Sprintf(
		`{ findByID(id: "%s") { members { ... on WorkspaceUserMember { userId user { metadata { photoURL } } } } } }`, f.ws))

	got := map[string]string{}
	for _, m := range data["findByID"].(map[string]any)["members"].([]any) {
		mm := m.(map[string]any)
		got[mm["userId"].(string)] = photoOf(mm["user"])
	}
	require.Len(t, got, 3)
	assertSigned(t, got[f.withPhoto.String()], "users/owner.webp")
	assertSigned(t, got[f.viewer.String()], "users/viewer.webp")
	assert.Empty(t, got[f.noPhoto.String()])
}

func TestGQL_MeSignsPhotoURL(t *testing.T) {
	e, f := startPhotoServer(t)
	data := photoQuery(t, e, f.viewer, `{ me { metadata { photoURL } } }`)
	assertSigned(t, photoOf(data["me"]), "users/viewer.webp")

	e, f = startPhotoServer(t)
	data = photoQuery(t, e, f.noPhoto, `{ me { metadata { photoURL } } }`)
	assert.Empty(t, photoOf(data["me"]))
}
