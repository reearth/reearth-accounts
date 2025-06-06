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

var (
	uId  = id.NewUserID()
	uId2 = id.NewUserID()
	uId3 = id.NewUserID()
	wId  = id.NewWorkspaceID()
	wId2 = id.NewWorkspaceID()
	iId  = id.NewIntegrationID()
	iId2 = id.NewIntegrationID()
	iId3 = id.NewIntegrationID()
)

func baseSeederWorkspace(ctx context.Context, r *repo.Container) error {
	u := user.New().ID(uId).
		Name("e2e").
		Email("e2e@e2e.com").
		Workspace(wId).
		MustBuild()
	if err := r.User.Save(ctx, u); err != nil {
		return err
	}
	u2 := user.New().ID(uId2).
		Name("e2e2").
		Email("e2e2@e2e.com").
		Workspace(wId2).
		MustBuild()
	if err := r.User.Save(ctx, u2); err != nil {
		return err
	}
	u3 := user.New().ID(uId3).
		Name("e2e3").
		Email("e2e3@e2e.com").
		Workspace(wId2).
		MustBuild()
	if err := r.User.Save(ctx, u3); err != nil {
		return err
	}
	roleOwner := workspace.Member{
		Role:      workspace.RoleOwner,
		InvitedBy: uId2,
	}
	roleReader := workspace.Member{
		Role:      workspace.RoleReader,
		InvitedBy: uId,
	}

	w := workspace.New().ID(wId).
		Name("e2e").
		Members(map[idx.ID[id.User]]workspace.Member{
			uId: roleOwner,
		}).
		Integrations(map[idx.ID[id.Integration]]workspace.Member{
			iId:  roleOwner,
			iId3: roleReader,
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

type GraphQLRequest struct {
	OperationName string         `json:"operationName"`
	Query         string         `json:"query"`
	Variables     map[string]any `json:"variables"`
}

func TestCreateWorkspace(t *testing.T) {
	e, _ := StartServer(t, &app.Config{}, true, baseSeederWorkspace)

	query := `mutation { createWorkspace(input: {name: "test"}){ workspace{ id name } }}`
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
		WithBytes(jsonData).Expect().Status(http.StatusOK).JSON().Object()
	o.Value("data").Object().Value("createWorkspace").Object().Value("workspace").Object().Value("name").String().IsEqual("test")
}

func TestDeleteWorkspace(t *testing.T) {
	e, r := StartServer(t, &app.Config{}, true, baseSeederWorkspace)
	_, err := r.Workspace.FindByID(context.Background(), wId)
	assert.Nil(t, err)
	query := fmt.Sprintf(`mutation { deleteWorkspace(input: {workspaceId: "%s"}){ workspaceId }}`, wId)
	request := GraphQLRequest{
		Query: query,
	}
	jsonData, err := json.Marshal(request)
	assert.Nil(t, err)

	o := e.POST("/api/graphql").
		WithHeader("authorization", "Bearer test").
		WithHeader("Content-Type", "application/json").
		WithHeader("X-Reearth-Debug-User", uId.String()).
		WithBytes(jsonData).Expect().Status(http.StatusOK).JSON().Object()
	o.Value("data").Object().Value("deleteWorkspace").Object().Value("workspaceId").String().IsEqual(wId.String())

	_, err = r.Workspace.FindByID(context.Background(), wId)
	assert.Equal(t, rerror.ErrNotFound, err)
}

func TestUpdateWorkspace(t *testing.T) {
	e, r := StartServer(t, &app.Config{}, true, baseSeederWorkspace)

	w, err := r.Workspace.FindByID(context.Background(), wId)
	assert.Nil(t, err)
	assert.Equal(t, "e2e", w.Name())

	query := fmt.Sprintf(`mutation { updateWorkspace(input: {workspaceId: "%s",name: "%s"}){ workspace{ id name } }}`, wId, "updated")
	request := GraphQLRequest{
		Query: query,
	}
	jsonData, err := json.Marshal(request)
	if err != nil {
		assert.Nil(t, err)
	}
	o := e.POST("/api/graphql").
		WithHeader("authorization", "Bearer test").
		WithHeader("Content-Type", "application/json").
		WithHeader("X-Reearth-Debug-User", uId.String()).
		WithBytes(jsonData).Expect().Status(http.StatusOK).JSON().Object()
	o.Value("data").Object().Value("updateWorkspace").Object().Value("workspace").Object().Value("name").String().IsEqual("updated")

	w, err = r.Workspace.FindByID(context.Background(), wId)
	assert.Nil(t, err)
	assert.Equal(t, "updated", w.Name())
}

func TestAddUsersToWorkspace(t *testing.T) {
	e, r := StartServer(t, &app.Config{}, true, baseSeederWorkspace)

	w, err := r.Workspace.FindByID(context.Background(), wId)
	assert.Nil(t, err)
	assert.False(t, w.Members().HasUser(uId2))

	query := fmt.Sprintf(`mutation { addUsersToWorkspace(input: {workspaceId: "%s", users: [{userId: "%s", role: READER}]}){ workspace{ id } }}`, wId, uId2)
	request := GraphQLRequest{
		Query: query,
	}
	jsonData, err := json.Marshal(request)
	if err != nil {
		assert.Nil(t, err)
	}
	e.POST("/api/graphql").
		WithHeader("authorization", "Bearer test").
		WithHeader("Content-Type", "application/json").
		WithHeader("X-Reearth-Debug-User", uId.String()).
		WithBytes(jsonData).Expect().Status(http.StatusOK)

	w, err = r.Workspace.FindByID(context.Background(), wId)
	assert.Nil(t, err)
	assert.True(t, w.Members().HasUser(uId2))
	assert.Equal(t, w.Members().User(uId2).Role, workspace.RoleReader)
}

func TestRemoveUserFromWorkspace(t *testing.T) {
	e, r := StartServer(t, &app.Config{}, true, baseSeederWorkspace)

	w, err := r.Workspace.FindByID(context.Background(), wId2)
	assert.Nil(t, err)
	assert.True(t, w.Members().HasUser(uId3))

	query := fmt.Sprintf(`mutation { removeUserFromWorkspace(input: {workspaceId: "%s", userId: "%s"}){ workspace{ id } }}`, wId2, uId3)
	request := GraphQLRequest{
		Query: query,
	}
	jsonData, err := json.Marshal(request)
	if err != nil {
		assert.Nil(t, err)
	}
	e.POST("/api/graphql").
		WithHeader("authorization", "Bearer test").
		WithHeader("Content-Type", "application/json").
		WithHeader("X-Reearth-Debug-User", uId.String()).
		WithBytes(jsonData).Expect().Status(http.StatusOK)

	w, err = r.Workspace.FindByID(context.Background(), wId)
	assert.Nil(t, err)
	assert.False(t, w.Members().HasUser(uId3))
}

func TestUpdateMemberOfWorkspace(t *testing.T) {
	e, r := StartServer(t, &app.Config{}, true, baseSeederWorkspace)

	w, err := r.Workspace.FindByID(context.Background(), wId2)
	assert.Nil(t, err)
	assert.Equal(t, w.Members().User(uId3).Role, workspace.RoleReader)
	query := fmt.Sprintf(`mutation { updateUserOfWorkspace(input: {workspaceId: "%s", userId: "%s", role: MAINTAINER}){ workspace{ id } }}`, wId2, uId3)
	request := GraphQLRequest{
		Query: query,
	}
	jsonData, err := json.Marshal(request)
	if err != nil {
		assert.Nil(t, err)
	}
	e.POST("/api/graphql").
		WithHeader("authorization", "Bearer test").
		WithHeader("Content-Type", "application/json").
		WithHeader("X-Reearth-Debug-User", uId.String()).
		WithBytes(jsonData).Expect().Status(http.StatusOK)

	w, err = r.Workspace.FindByID(context.Background(), wId2)
	assert.Nil(t, err)
	assert.Equal(t, w.Members().User(uId3).Role, workspace.RoleMaintainer)
}

func TestAddIntegrationToWorkspace(t *testing.T) {
	e, r := StartServer(t, &app.Config{}, true, baseSeederWorkspace)

	w, err := r.Workspace.FindByID(context.Background(), wId)
	assert.Nil(t, err)
	assert.False(t, w.Members().HasUser(uId2))

	query := fmt.Sprintf(`mutation { addIntegrationToWorkspace(input: {workspaceId: "%s", integrationId: "%s",  role: READER}){ workspace{ id } }}`, wId, iId2)
	request := GraphQLRequest{
		Query: query,
	}
	jsonData, err := json.Marshal(request)
	if err != nil {
		assert.Nil(t, err)
	}
	e.POST("/api/graphql").
		WithHeader("authorization", "Bearer test").
		WithHeader("Content-Type", "application/json").
		WithHeader("X-Reearth-Debug-User", uId.String()).
		WithBytes(jsonData).Expect().Status(http.StatusOK)

	w, err = r.Workspace.FindByID(context.Background(), wId)
	assert.Nil(t, err)
	assert.True(t, w.Members().HasIntegration(iId2))
	assert.Equal(t, w.Members().Integration(iId2).Role, workspace.RoleReader)
}

func TestRemoveIntegrationFromWorkspace(t *testing.T) {
	e, r := StartServer(t, &app.Config{}, true, baseSeederWorkspace)

	w, err := r.Workspace.FindByID(context.Background(), wId)
	assert.Nil(t, err)
	assert.True(t, w.Members().HasIntegration(iId))

	query := fmt.Sprintf(`mutation { removeIntegrationFromWorkspace(input: {workspaceId: "%s", integrationId: "%s"}){ workspace{ id } }}`, wId, iId)
	request := GraphQLRequest{
		Query: query,
	}
	jsonData, err := json.Marshal(request)
	if err != nil {
		assert.Nil(t, err)
	}
	e.POST("/api/graphql").
		WithHeader("authorization", "Bearer test").
		WithHeader("Content-Type", "application/json").
		WithHeader("X-Reearth-Debug-User", uId.String()).
		WithBytes(jsonData).Expect().Status(http.StatusOK)

	w, err = r.Workspace.FindByID(context.Background(), wId)
	assert.Nil(t, err)
	assert.False(t, w.Members().HasIntegration(iId))
}

func TestUpdateIntegrationOfWorkspace(t *testing.T) {
	e, r := StartServer(t, &app.Config{}, true, baseSeederWorkspace)

	w, err := r.Workspace.FindByID(context.Background(), wId)
	assert.Nil(t, err)
	assert.Equal(t, w.Members().Integration(iId3).Role, workspace.RoleReader)
	query := fmt.Sprintf(`mutation { updateIntegrationOfWorkspace(input: {workspaceId: "%s", integrationId: "%s", role: MAINTAINER}){ workspace{ id } }}`, wId, iId3)
	request := GraphQLRequest{
		Query: query,
	}
	jsonData, err := json.Marshal(request)
	if err != nil {
		assert.Nil(t, err)
	}
	e.POST("/api/graphql").
		WithHeader("authorization", "Bearer test").
		WithHeader("Content-Type", "application/json").
		WithHeader("X-Reearth-Debug-User", uId.String()).
		WithBytes(jsonData).Expect().Status(http.StatusOK)

	w, err = r.Workspace.FindByID(context.Background(), wId)
	assert.Nil(t, err)
	assert.Equal(t, w.Members().Integration(iId3).Role, workspace.RoleMaintainer)
}
