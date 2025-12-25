package workspace

import (
	"context"
	"errors"
	"strings"

	"github.com/hasura/go-graphql-client"
	"github.com/reearth/reearth-accounts/server/pkg/gqlclient/gqlerror"
	"github.com/reearth/reearth-accounts/server/pkg/gqlclient/gqlmodel"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
)

var (
	ErrWorkspaceNotFound = func(err error) bool {
		var gqlErrs graphql.Errors
		if errors.As(err, &gqlErrs) {
			for _, gqlErr := range gqlErrs {
				if strings.Contains(gqlErr.Message, "not found") {
					return true
				}
			}
		}

		return false
	}

	ErrMemberAlreadyJoined = func(err error) bool {
		var gqlErrs graphql.Errors
		if errors.As(err, &gqlErrs) {
			for _, gqlErr := range gqlErrs {
				if strings.Contains(gqlErr.Message, "user already joined") {
					return true
				}
			}
		}

		return false
	}

	ErrUserIsNotMember = func(err error) bool {
		var gqlErrs graphql.Errors
		if errors.As(err, &gqlErrs) {
			for _, gqlErr := range gqlErrs {
				if strings.Contains(gqlErr.Message, "target user does not exist in the workspace") {
					return true
				}
			}
		}

		return false
	}
)

type workspaceRepo struct {
	client *graphql.Client
}

type WorkspaceRepo interface {
	FindByUser(ctx context.Context, userID string) (workspace.List, error)
	FindByID(ctx context.Context, id string) (*workspace.Workspace, error)
	FindByAlias(ctx context.Context, alias string) (*workspace.Workspace, error)
	FindByUserWithPagination(ctx context.Context, userID string, page int64, size int64) (workspace.List, int, error)
	CreateWorkspace(ctx context.Context, input CreateWorkspaceInput) (*workspace.Workspace, error)
	UpdateWorkspace(ctx context.Context, input UpdateWorkspaceInput) (*workspace.Workspace, error)
	DeleteWorkspace(ctx context.Context, workspaceID string) error
	AddUsersToWorkspace(ctx context.Context, input AddUsersToWorkspaceInput) (*workspace.Workspace, error)
	RemoveUserFromWorkspace(ctx context.Context, workspaceID, userID string) (*workspace.Workspace, error)
	UpdateUserOfWorkspace(ctx context.Context, input UpdateUserOfWorkspaceInput) (*workspace.Workspace, error)
}

// Input types for mutations
type CreateWorkspaceInput struct {
	Alias       string
	Name        string
	Description *string
}

type UpdateWorkspaceInput struct {
	WorkspaceID string
	Name        string
}

type MemberInput struct {
	UserID string
	Role   string
}

type AddUsersToWorkspaceInput struct {
	WorkspaceID string
	Users       []MemberInput
}

type UpdateUserOfWorkspaceInput struct {
	WorkspaceID string
	UserID      string
	Role        string
}

func NewRepo(gql *graphql.Client) WorkspaceRepo {
	return &workspaceRepo{client: gql}
}

func (r *workspaceRepo) FindByUser(ctx context.Context, userID string) (workspace.List, error) {
	var q findByUserQuery
	vars := map[string]interface{}{
		"userId": graphql.ID(userID),
	}
	if err := r.client.Query(ctx, &q, vars); err != nil {
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	return gqlmodel.ToWorkspaces(ctx, q.FindByUser), nil
}

func (r *workspaceRepo) FindByID(ctx context.Context, id string) (*workspace.Workspace, error) {
	var q findByIDQuery
	vars := map[string]interface{}{
		"id": graphql.ID(id),
	}
	if err := r.client.Query(ctx, &q, vars); err != nil {
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	return gqlmodel.ToWorkspace(ctx, q.Workspace)
}

func (r *workspaceRepo) FindByAlias(ctx context.Context, alias string) (*workspace.Workspace, error) {
	var q findByAliasQuery
	vars := map[string]interface{}{
		"alias": graphql.String(alias),
	}
	if err := r.client.Query(ctx, &q, vars); err != nil {
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	return gqlmodel.ToWorkspace(ctx, q.Workspace)
}

func (r *workspaceRepo) FindByUserWithPagination(ctx context.Context, userID string, page int64, size int64) (workspace.List, int, error) {
	var q FindByUserWithPaginationQuery
	vars := map[string]interface{}{
		"userId": graphql.ID(userID),
		"page":   graphql.Int(page),
		"size":   graphql.Int(size),
	}
	if err := r.client.Query(ctx, &q, vars); err != nil {
		return nil, 0, gqlerror.ReturnAccountsError(ctx, err)
	}

	workspaces := gqlmodel.ToWorkspaces(ctx, q.FindByUserWithPagination.Workspaces)
	return workspaces, q.FindByUserWithPagination.TotalCount, nil
}

func (r *workspaceRepo) CreateWorkspace(ctx context.Context, input CreateWorkspaceInput) (*workspace.Workspace, error) {
	var m createWorkspaceMutation

	// Use individual variables instead of input object to avoid type inference issues
	vars := map[string]interface{}{
		"alias":       graphql.String(input.Alias),
		"name":        graphql.String(input.Name),
		"description": (*graphql.String)(nil), // default to null
	}

	// Only set description if it's not nil
	if input.Description != nil {
		vars["description"] = graphql.String(*input.Description)
	}

	if err := r.client.Mutate(ctx, &m, vars); err != nil {
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	return toWorkspace(ctx, m.CreateWorkspace.Workspace.ID, m.CreateWorkspace.Workspace.Name, m.CreateWorkspace.Workspace.Alias, m.CreateWorkspace.Workspace.Personal)
}

func (r *workspaceRepo) UpdateWorkspace(ctx context.Context, input UpdateWorkspaceInput) (*workspace.Workspace, error) {
	var m updateWorkspaceMutation
	vars := map[string]interface{}{
		"workspaceId": graphql.ID(input.WorkspaceID),
		"name":        graphql.String(input.Name),
	}
	if err := r.client.Mutate(ctx, &m, vars); err != nil {
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	return toWorkspace(ctx, m.UpdateWorkspace.Workspace.ID, m.UpdateWorkspace.Workspace.Name, m.UpdateWorkspace.Workspace.Alias, m.UpdateWorkspace.Workspace.Personal)
}

func (r *workspaceRepo) DeleteWorkspace(ctx context.Context, workspaceID string) error {
	var m deleteWorkspaceMutation
	vars := map[string]interface{}{
		"workspaceId": graphql.ID(workspaceID),
	}
	if err := r.client.Mutate(ctx, &m, vars); err != nil {
		return gqlerror.ReturnAccountsError(ctx, err)
	}

	return nil
}

func (r *workspaceRepo) AddUsersToWorkspace(ctx context.Context, input AddUsersToWorkspaceInput) (*workspace.Workspace, error) {
	// Convert MemberInput to GraphQL format
	users := make([]map[string]interface{}, len(input.Users))
	for i, user := range input.Users {
		users[i] = map[string]interface{}{
			"userId": graphql.ID(user.UserID),
			"role":   graphql.String(user.Role),
		}
	}

	var m addUsersToWorkspaceMutation
	vars := map[string]interface{}{
		"workspaceId": graphql.ID(input.WorkspaceID),
		"users":       users,
	}
	if err := r.client.Mutate(ctx, &m, vars); err != nil {
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	return toWorkspace(ctx, m.AddUsersToWorkspace.Workspace.ID, m.AddUsersToWorkspace.Workspace.Name, m.AddUsersToWorkspace.Workspace.Alias, m.AddUsersToWorkspace.Workspace.Personal)
}

func (r *workspaceRepo) RemoveUserFromWorkspace(ctx context.Context, workspaceID, userID string) (*workspace.Workspace, error) {
	var m removeUserFromWorkspaceMutation
	vars := map[string]interface{}{
		"workspaceId": graphql.ID(workspaceID),
		"userId":      graphql.ID(userID),
	}
	if err := r.client.Mutate(ctx, &m, vars); err != nil {
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	return toWorkspace(ctx, m.RemoveUserFromWorkspace.Workspace.ID, m.RemoveUserFromWorkspace.Workspace.Name, m.RemoveUserFromWorkspace.Workspace.Alias, m.RemoveUserFromWorkspace.Workspace.Personal)
}

func (r *workspaceRepo) UpdateUserOfWorkspace(ctx context.Context, input UpdateUserOfWorkspaceInput) (*workspace.Workspace, error) {
	var m updateUserOfWorkspaceMutation
	vars := map[string]interface{}{
		"workspaceId": graphql.ID(input.WorkspaceID),
		"userId":      graphql.ID(input.UserID),
		"role":        graphql.String(input.Role),
	}
	if err := r.client.Mutate(ctx, &m, vars); err != nil {
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	return toWorkspace(ctx, m.UpdateUserOfWorkspace.Workspace.ID, m.UpdateUserOfWorkspace.Workspace.Name, m.UpdateUserOfWorkspace.Workspace.Alias, m.UpdateUserOfWorkspace.Workspace.Personal)
}

// toWorkspace converts GraphQL types to full Workspace domain object
func toWorkspace(ctx context.Context, id graphql.ID, name graphql.String, alias graphql.String, personal bool) (*workspace.Workspace, error) {
	wsID, err := workspace.IDFrom(string(id))
	if err != nil {
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	return workspace.New().
		ID(wsID).
		Name(string(name)).
		Alias(string(alias)).
		Members(make(map[workspace.UserID]workspace.Member)). // Empty members map for mutations
		Metadata(workspace.MetadataFrom("", "", "", "", "")).  // Empty metadata for mutations
		Personal(personal).
		MustBuild(), nil
}
