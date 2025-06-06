package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/reearth/reearth-accounts/internal/app"
	"github.com/reearth/reearth-accounts/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/pkg/id"
	"github.com/reearth/reearth-accounts/pkg/user"
	"github.com/reearth/reearth-accounts/pkg/workspace"
	"github.com/reearth/reearthx/idx"
	"github.com/reearth/reearthx/rerror"
	"github.com/stretchr/testify/assert"
)

func baseSeederOneUser(ctx context.Context, r *repo.Container) error {
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
		Role:      workspace.RoleOwner,
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

func baseSeederUser(ctx context.Context, r *repo.Container) error {
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

	return nil
}

func TestUpdateMe(t *testing.T) {
	e, _ := StartServer(t, &app.Config{}, true, baseSeederUser)
	query := `mutation { updateMe(input: {name: "updated",email:"hoge@test.com",lang:"ja",theme:DEFAULT,password: "Ajsownndww1",passwordConfirmation: "Ajsownndww1"}){ me{ id name email metadata { lang theme } } }}`
	request := GraphQLRequest{
		Query: query,
	}
	jsonData, err := json.Marshal(request)
	if err != nil {
		assert.NoError(t, err)
	}
	o := e.POST("/api/graphql").
		WithHeader("authorization", "Bearer test").
		WithHeader("Content-Type", "application/json").
		WithHeader("X-Reearth-Debug-User", uId.String()).
		WithBytes(jsonData).Expect().Status(http.StatusOK).JSON().Object().Value("data").Object().Value("updateMe").Object().Value("me").Object()
	o.Value("name").String().IsEqual("updated")
	o.Value("email").String().IsEqual("hoge@test.com")
	o.Value("metadata").Object().Value("lang").String().IsEqual("ja")
	o.Value("metadata").Object().Value("theme").String().IsEqual("default")
}

func TestRemoveMyAuth(t *testing.T) {
	e, r := StartServer(t, &app.Config{}, true, baseSeederUser)

	u, err := r.User.FindByID(context.Background(), uId)
	assert.Nil(t, err)
	assert.Equal(t, &user.Auth{Provider: "reearth", Sub: "reearth|" + uId.String()}, u.Auths().GetByProvider("reearth"))

	query := `mutation { removeMyAuth(input: {auth: "reearth"}){ me{ id name email metadata { lang theme } } }}`
	request := GraphQLRequest{
		Query: query,
	}
	jsonData, err := json.Marshal(request)
	if err != nil {
		assert.NoError(t, err)
	}
	e.POST("/api/graphql").
		WithHeader("authorization", "Bearer test").
		WithHeader("Content-Type", "application/json").
		WithHeader("X-Reearth-Debug-User", uId.String()).
		WithBytes(jsonData).Expect().Status(http.StatusOK).JSON().Object()

	u, err = r.User.FindByID(context.Background(), uId)
	assert.Nil(t, err)
	assert.Nil(t, u.Auths().Get("sub"))
}

func TestDeleteMe(t *testing.T) {
	e, r := StartServer(t, &app.Config{}, true, baseSeederUser)

	u, err := r.User.FindByID(context.Background(), uId)
	assert.Nil(t, err)
	assert.NotNil(t, u)

	query := fmt.Sprintf(`mutation { deleteMe(input: {userId: "%s"}){ userId }}`, uId)
	request := GraphQLRequest{
		Query: query,
	}
	jsonData, err := json.Marshal(request)
	if err != nil {
		assert.NoError(t, err)
	}
	e.POST("/api/graphql").
		WithHeader("authorization", "Bearer test").
		WithHeader("Content-Type", "application/json").
		WithHeader("X-Reearth-Debug-User", uId.String()).
		WithBytes(jsonData).Expect().Status(http.StatusOK).JSON().Object()

	_, err = r.User.FindByID(context.Background(), uId)
	assert.Equal(t, rerror.ErrNotFound, err)
}

func TestMe(t *testing.T) {
	e, _ := StartServer(t, &app.Config{}, true, baseSeederUser)
	query := ` { me{ id name email metadata { lang theme } myWorkspaceId } }`
	request := GraphQLRequest{
		Query: query,
	}
	jsonData, err := json.Marshal(request)
	if err != nil {
		assert.NoError(t, err)
	}
	o := e.POST("/api/graphql").
		WithHeader("authorization", "Bearer test").
		WithHeader("Content-Type", "application/json").
		WithHeader("X-Reearth-Debug-User", uId.String()).
		WithBytes(jsonData).Expect().Status(http.StatusOK).JSON().Object().Value("data").Object().Value("me").Object()
	o.Value("id").String().IsEqual(uId.String())
	o.Value("name").String().IsEqual("e2e")
	o.Value("email").String().IsEqual("e2e@e2e.com")
	o.Value("metadata").Object().Value("lang").String().IsEqual("ja")
	o.Value("metadata").Object().Value("theme").String().IsEqual("dark")
	o.Value("myWorkspaceId").String().IsEqual(wId.String())
}

func TestUserByNameOrEmail(t *testing.T) {
	e, _ := StartServer(t, &app.Config{}, true, baseSeederUser)
	query := fmt.Sprintf(` { userByNameOrEmail(nameOrEmail: "%s"){ id name email } }`, "e2e")
	request := GraphQLRequest{
		Query: query,
	}
	jsonData, err := json.Marshal(request)
	if err != nil {
		assert.NoError(t, err)
	}
	o := e.POST("/api/graphql").
		WithHeader("authorization", "Bearer test").
		WithHeader("Content-Type", "application/json").
		WithHeader("X-Reearth-Debug-User", uId.String()).
		WithBytes(jsonData).Expect().Status(http.StatusOK).JSON().Object().Value("data").Object().Value("userByNameOrEmail").Object()
	o.Value("id").String().IsEqual(uId.String())
	o.Value("name").String().IsEqual("e2e")
	o.Value("email").String().IsEqual("e2e@e2e.com")
}

func TestNode(t *testing.T) {
	e, _ := StartServer(t, &app.Config{}, true, baseSeederUser)
	query := fmt.Sprintf(` { node(id: "%s", type: USER){ id } }`, uId.String())
	request := GraphQLRequest{
		Query: query,
	}
	jsonData, err := json.Marshal(request)
	if err != nil {
		assert.NoError(t, err)
	}
	o := e.POST("/api/graphql").
		WithHeader("authorization", "Bearer test").
		WithHeader("Content-Type", "application/json").
		WithHeader("X-Reearth-Debug-User", uId.String()).
		WithBytes(jsonData).Expect().Status(http.StatusOK).JSON().Object().Value("data").Object().Value("node").Object()
	o.Value("id").String().IsEqual(uId.String())
}

func TestNodes(t *testing.T) {
	e, _ := StartServer(t, &app.Config{}, true, baseSeederUser)
	query := fmt.Sprintf(` { nodes(id: "%s", type: USER){ id } }`, uId.String())
	request := GraphQLRequest{
		Query: query,
	}
	jsonData, err := json.Marshal(request)
	if err != nil {
		assert.NoError(t, err)
	}
	o := e.POST("/api/graphql").
		WithHeader("authorization", "Bearer test").
		WithHeader("Content-Type", "application/json").
		WithHeader("X-Reearth-Debug-User", uId.String()).
		WithBytes(jsonData).Expect().Status(http.StatusOK).JSON().Object().Value("data").Object().Value("nodes")
	o.Array().ConsistsOf(map[string]string{"id": uId.String()})
}
