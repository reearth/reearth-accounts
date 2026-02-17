//go:generate go run github.com/Khan/genqlient

package proxy

import (
	"context"

	_ "github.com/Khan/genqlient/generate"
	"github.com/Khan/genqlient/graphql"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	accountid "github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/samber/lo"
)

type Workspace struct {
	http     HTTPClient
	gql      graphql.Client
	endpoint string
}

func NewWorkspace(endpoint string, h HTTPClient) interfaces.Workspace {
	return &Workspace{
		http:     h,
		endpoint: endpoint,
		gql:      graphql.NewClient(endpoint, h),
	}
}

func (w *Workspace) Fetch(ctx context.Context, ids workspace.IDList, op *workspace.Operator) (workspace.List, error) {
	return WorkspaceByIDsResponseTo(WorkspaceByIDs(ctx, w.gql, ids.Strings()))
}

func (w *Workspace) FetchByID(ctx context.Context, id workspace.ID) (*workspace.Workspace, error) {
	res, err := FindByID(ctx, w.gql, id.String())
	if err != nil {
		return nil, err
	}
	return ToWorkspace(res.FindByID.FragmentWorkspace)
}

func (w *Workspace) FetchByName(ctx context.Context, name string) (*workspace.Workspace, error) {
	res, err := FindByName(ctx, w.gql, name)
	if err != nil {
		return nil, err
	}
	return ToWorkspace(res.FindByName.FragmentWorkspace)
}

func (w *Workspace) FetchByAlias(ctx context.Context, alias string) (*workspace.Workspace, error) {
	res, err := FindByAlias(ctx, w.gql, alias)
	if err != nil {
		return nil, err
	}
	return ToWorkspace(res.FindByAlias.FragmentWorkspace)
}

func (w *Workspace) FindByUser(ctx context.Context, userID accountid.UserID, op *workspace.Operator) (workspace.List, error) {
	res, err := FindByUser(ctx, w.gql, userID.String())
	if err != nil {
		return nil, err
	}
	ws := make([]FragmentWorkspace, len(res.FindByUser))
	for i, w := range res.FindByUser {
		ws[i] = w.FragmentWorkspace
	}
	return ToWorkspaces(ws)
}

func (w *Workspace) FetchByUserWithPagination(ctx context.Context, userID accountid.UserID, param interfaces.FetchByUserWithPaginationParam) (interfaces.FetchByUserWithPaginationResult, error) {
	res, err := FindByUserWithPagination(ctx, w.gql, userID.String(), int(param.Page), int(param.Size))
	if err != nil {
		return interfaces.FetchByUserWithPaginationResult{}, err
	}
	workspaces := make([]FragmentWorkspace, len(res.FindByUserWithPagination.Workspaces))
	for i, w := range res.FindByUserWithPagination.Workspaces {
		workspaces[i] = w.FragmentWorkspace
	}
	ws, err := ToWorkspaces(workspaces)
	if err != nil {
		return interfaces.FetchByUserWithPaginationResult{}, err
	}
	return interfaces.FetchByUserWithPaginationResult{
		Workspaces: ws,
		TotalCount: res.FindByUserWithPagination.TotalCount,
	}, nil
}

func (w *Workspace) Create(ctx context.Context, alias, name, description string, userID accountid.UserID, op *workspace.Operator) (*workspace.Workspace, error) {
	res, err := CreateWorkspace(ctx, w.gql, CreateWorkspaceInput{
		Alias:       alias,
		Name:        name,
		Description: description,
	})
	if err != nil {
		return nil, err
	}
	return ToWorkspace(res.CreateWorkspace.Workspace.FragmentWorkspace)
}

func (w *Workspace) Update(ctx context.Context, param interfaces.UpdateWorkspaceParam, op *workspace.Operator) (*workspace.Workspace, error) {
	// Note: File upload (param.FileImage) is not supported via proxy as it requires multipart form data
	// Use param.PhotoURL to pass the photo path/URL directly for service-to-service communication
	res, err := UpdateWorkspace(ctx, w.gql, param.ID.String(), param.Name, param.Alias, param.Description, param.Website, param.PhotoURL)
	if err != nil {
		return nil, err
	}
	return ToWorkspace(res.UpdateWorkspace.Workspace.FragmentWorkspace)
}

func (w *Workspace) AddUserMember(ctx context.Context, id workspace.ID, users map[accountid.UserID]role.RoleType, op *workspace.Operator) (*workspace.Workspace, error) {
	members := []MemberInput{}
	for id, role := range users {
		members = append(members, MemberInput{UserId: id.String(), Role: Role(string(role))})
	}
	res, err := AddUsersToWorkspace(ctx, w.gql, AddUsersToWorkspaceInput{WorkspaceId: id.String(), Users: members})
	if err != nil {
		return nil, err
	}
	return ToWorkspace(res.AddUsersToWorkspace.Workspace.FragmentWorkspace)
}

func (w *Workspace) AddIntegrationMember(ctx context.Context, id workspace.ID, integrationId workspace.IntegrationID, role role.RoleType, op *workspace.Operator) (*workspace.Workspace, error) {
	res, err := AddIntegrationToWorkspace(ctx, w.gql, AddIntegrationToWorkspaceInput{WorkspaceId: id.String(), IntegrationId: integrationId.String(), Role: Role(string(role))})
	if err != nil {
		return nil, err
	}
	return ToWorkspace(res.AddIntegrationToWorkspace.Workspace.FragmentWorkspace)
}

func (w *Workspace) UpdateUserMember(ctx context.Context, id workspace.ID, userID accountid.UserID, role role.RoleType, op *workspace.Operator) (*workspace.Workspace, error) {
	res, err := UpdateUserOfWorkspace(ctx, w.gql, UpdateUserOfWorkspaceInput{WorkspaceId: id.String(), UserId: userID.String(), Role: Role(string(role))})
	if err != nil {
		return nil, err
	}
	return ToWorkspace(res.UpdateUserOfWorkspace.Workspace.FragmentWorkspace)
}

func (w *Workspace) UpdateIntegration(ctx context.Context, id workspace.ID, integrationID workspace.IntegrationID, role role.RoleType, op *workspace.Operator) (*workspace.Workspace, error) {
	res, err := UpdateIntegrationOfWorkspace(ctx, w.gql, UpdateIntegrationOfWorkspaceInput{WorkspaceId: id.String(), IntegrationId: integrationID.String(), Role: Role(string(role))})
	if err != nil {
		return nil, err
	}
	return ToWorkspace(res.UpdateIntegrationOfWorkspace.Workspace.FragmentWorkspace)
}

func (w *Workspace) RemoveUserMember(ctx context.Context, id workspace.ID, userID accountid.UserID, op *workspace.Operator) (*workspace.Workspace, error) {
	res, err := RemoveUserFromWorkspace(ctx, w.gql, RemoveUserFromWorkspaceInput{WorkspaceId: id.String(), UserId: userID.String()})
	if err != nil {
		return nil, err
	}
	return ToWorkspace(res.RemoveUserFromWorkspace.Workspace.FragmentWorkspace)
}

func (w *Workspace) RemoveMultipleUserMembers(ctx context.Context, id workspace.ID, userIDs accountid.UserIDList, op *workspace.Operator) (*workspace.Workspace, error) {
	res, err := RemoveMultipleUsersFromWorkspace(ctx, w.gql, RemoveMultipleUsersFromWorkspaceInput{WorkspaceId: id.String(), UserIds: lo.Map(userIDs, func(u accountid.UserID, _ int) string { return u.String() })})
	if err != nil {
		return nil, err
	}
	return ToWorkspace(res.RemoveMultipleUsersFromWorkspace.Workspace.FragmentWorkspace)
}

func (w *Workspace) RemoveIntegration(ctx context.Context, id workspace.ID, integrationID workspace.IntegrationID, op *workspace.Operator) (*workspace.Workspace, error) {
	res, err := RemoveIntegrationFromWorkspace(ctx, w.gql, RemoveIntegrationFromWorkspaceInput{WorkspaceId: id.String(), IntegrationId: integrationID.String()})
	if err != nil {
		return nil, err
	}
	return ToWorkspace(res.RemoveIntegrationFromWorkspace.Workspace.FragmentWorkspace)
}

func (w *Workspace) RemoveIntegrations(ctx context.Context, id workspace.ID, integrationIDs workspace.IntegrationIDList, op *workspace.Operator) (*workspace.Workspace, error) {
	res, err := RemoveIntegrationsFromWorkspace(ctx, w.gql, RemoveIntegrationsFromWorkspaceInput{WorkspaceId: id.String(), IntegrationIds: lo.Map(integrationIDs, func(i workspace.IntegrationID, _ int) string { return i.String() })})
	if err != nil {
		return nil, err
	}
	return ToWorkspace(res.RemoveIntegrationsFromWorkspace.Workspace.FragmentWorkspace)
}

func (w *Workspace) Remove(ctx context.Context, id workspace.ID, op *workspace.Operator) error {
	_, err := DeleteWorkspace(ctx, w.gql, DeleteWorkspaceInput{WorkspaceId: id.String()})
	if err != nil {
		return err
	}
	return nil
}

func (w *Workspace) TransferOwnership(ctx context.Context, id workspace.ID, newOwnerID accountid.UserID, op *workspace.Operator) (*workspace.Workspace, error) {
	res, err := TransferWorkspaceOwnership(ctx, w.gql, TransferWorkspaceOwnershipInput{WorkspaceId: id.String(), NewOwnerId: newOwnerID.String()})
	if err != nil {
		return nil, err
	}
	return ToWorkspace(res.TransferWorkspaceOwnership.Workspace.FragmentWorkspace)
}
