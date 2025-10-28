package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/app"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/idx"
	"github.com/stretchr/testify/assert"
)

func baseSeederGetMe(ctx context.Context, r *repo.Container) error {
	auth := user.ReearthSub(uId.String())
	metadata := user.NewMetadata()
	metadata.LangFrom("en")
	metadata.SetTheme(user.ThemeLight)
	metadata.SetDescription("Test user description")
	metadata.SetWebsite("https://example.com")

	u := user.New().ID(uId).
		Name("Test User").
		Alias("testuser").
		Email("test@example.com").
		Auths([]user.Auth{*auth}).
		Metadata(metadata).
		Workspace(wId).
		MustBuild()
	if err := r.User.Save(ctx, u); err != nil {
		return err
	}

	// Create additional user for workspace membership testing
	u2 := user.New().ID(uId2).
		Name("Other User").
		Email("other@example.com").
		Workspace(wId2).
		MustBuild()
	if err := r.User.Save(ctx, u2); err != nil {
		return err
	}

	roleOwner := workspace.Member{
		Role:      workspace.RoleOwner,
		InvitedBy: uId,
	}
	roleWriter := workspace.Member{
		Role:      workspace.RoleWriter,
		InvitedBy: uId,
	}
	roleReader := workspace.Member{
		Role:      workspace.RoleReader,
		InvitedBy: uId,
	}

	// Workspace 1: Personal workspace where user is owner
	w := workspace.New().ID(wId).
		Name("Personal Workspace").
		Alias("personal").
		Members(map[idx.ID[id.User]]workspace.Member{
			uId: roleOwner,
		}).
		Integrations(map[idx.ID[id.Integration]]workspace.Member{
			iId: roleOwner,
		}).
		Personal(true).
		MustBuild()
	if err := r.Workspace.Save(ctx, w); err != nil {
		return err
	}

	// Workspace 2: Team workspace where user is owner
	w2 := workspace.New().ID(wId2).
		Name("Team Workspace").
		Alias("team").
		Members(map[idx.ID[id.User]]workspace.Member{
			uId:  roleOwner,
			uId2: roleWriter,
		}).
		Integrations(map[idx.ID[id.Integration]]workspace.Member{
			iId: roleOwner,
		}).
		Personal(false).
		MustBuild()
	if err := r.Workspace.Save(ctx, w2); err != nil {
		return err
	}

	// Workspace 3: Project workspace where user is writer
	w3 := workspace.New().ID(wId3).
		Name("Project Workspace").
		Alias("project").
		Members(map[idx.ID[id.User]]workspace.Member{
			uId:  roleWriter,
			uId2: roleOwner,
		}).
		Integrations(map[idx.ID[id.Integration]]workspace.Member{
			iId: roleReader,
		}).
		Personal(false).
		MustBuild()
	if err := r.Workspace.Save(ctx, w3); err != nil {
		return err
	}

	return nil
}

func TestGetMeAllFields(t *testing.T) {
	e, _ := StartServer(t, &app.Config{}, true, baseSeederGetMe, nil)

	query := `{
  me {
    id
    name
    alias
    email
    metadata {
      description
      lang
      photoURL
      theme
      website
    }
    host
    myWorkspaceId
    auths
  }
}`
	request := GraphQLRequest{
		Query: query,
	}
	jsonData, err := json.Marshal(request)
	assert.NoError(t, err)

	o := e.POST("/api/graphql").
		WithHeader("authorization", "Bearer test").
		WithHeader("Content-Type", "application/json").
		WithHeader("X-Reearth-Debug-User", uId.String()).
		WithBytes(jsonData).
		Expect().Status(http.StatusOK).
		JSON().Object().
		Value("data").Object().
		Value("me").Object()

	o.Value("id").String().IsEqual(uId.String())
	o.Value("name").String().IsEqual("Test User")
	o.Value("alias").String().IsEqual("testuser")
	o.Value("email").String().IsEqual("test@example.com")
	o.Value("myWorkspaceId").String().IsEqual(wId.String())

	auths := o.Value("auths").Array()
	auths.Length().IsEqual(1)
	auths.Value(0).String().Contains("reearth")

	metadata := o.Value("metadata").Object()
	metadata.Value("description").String().IsEqual("Test user description")
	metadata.Value("lang").String().IsEqual("en")
	metadata.Value("theme").String().IsEqual("light")
	metadata.Value("website").String().IsEqual("https://example.com")
}

func TestGetMeMinimalFields(t *testing.T) {
	e, _ := StartServer(t, &app.Config{StorageIsLocal: true}, true, baseSeederGetMe, nil)

	query := `{
		me {
			id
			name
			email
		}
	}`
	request := GraphQLRequest{
		Query: query,
	}
	jsonData, err := json.Marshal(request)
	assert.NoError(t, err)

	o := e.POST("/api/graphql").
		WithHeader("authorization", "Bearer test").
		WithHeader("Content-Type", "application/json").
		WithHeader("X-Reearth-Debug-User", uId.String()).
		WithBytes(jsonData).
		Expect().Status(http.StatusOK).
		JSON().Object().
		Value("data").Object().
		Value("me").Object()

	o.Value("id").String().IsEqual(uId.String())
	o.Value("name").String().IsEqual("Test User")
	o.Value("email").String().IsEqual("test@example.com")
}
