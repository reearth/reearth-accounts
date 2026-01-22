//go:generate go run go.uber.org/mock/mockgen -source=repo.go -destination=mockrepo/mockrepo.go -package=mockrepo -mock_names=Repo=MockWorkspaceRepos
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
	Create(ctx context.Context, name string, alias string, description string) (*workspace.Workspace, error)
	Update(ctx context.Context, id string, alias string, name string, email string, website string, description string) error
	Delete(ctx context.Context, id string) error
	AddUser(ctx context.Context, id string, userID string, role workspace.Role) error
	RemoveUser(ctx context.Context, workspaceID string, userID string) error
	TransferOwnership(ctx context.Context, workspaceID string, newOwnerID string) (*workspace.Workspace, error)
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
		return nil, err
	}

	return gqlmodel.ToWorkspace(ctx, q.Workspace)
}

func (r *workspaceRepo) FindByAlias(ctx context.Context, alias string) (*workspace.Workspace, error) {
	var q findByAliasQuery
	vars := map[string]interface{}{
		"alias": graphql.String(alias),
	}
	if err := r.client.Query(ctx, &q, vars); err != nil {
		return nil, err
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
		return nil, 0, err
	}

	workspaces := gqlmodel.ToWorkspaces(ctx, q.FindByUserWithPagination.Workspaces)
	return workspaces, q.FindByUserWithPagination.TotalCount, nil
}

func (r *workspaceRepo) Create(ctx context.Context, name string, alias string, description string) (*workspace.Workspace, error) {
	var m createWorkspaceMutation
	vars := map[string]interface{}{
		"alias":       graphql.String(alias),
		"name":        graphql.String(name),
		"description": graphql.String(description),
	}

	if err := r.client.Mutate(ctx, &m, vars); err != nil {
		return nil, err
	}

	return gqlmodel.ToWorkspace(ctx, m.CreateWorkspace.Workspace)
}

func (r *workspaceRepo) Update(ctx context.Context, id string, alias string, name string, email string, website string, description string) error {
	var m updateWorkspaceMutation
	vars := map[string]interface{}{
		"workspaceId": graphql.ID(id),
		"name":        graphql.String(name),
	}

	return r.client.Mutate(ctx, &m, vars)
}

func (r *workspaceRepo) Delete(ctx context.Context, id string) error {
	var m deleteWorkspaceMutation
	vars := map[string]interface{}{
		"workspaceId": graphql.ID(id),
	}

	return r.client.Mutate(ctx, &m, vars)
}

func (r *workspaceRepo) AddUser(ctx context.Context, id string, userID string, role workspace.Role) error {
	var m addUsersToWorkspaceMutation
	users := []gqlmodel.MemberInput{
		{
			UserID: graphql.ID(userID),
			Role:   graphql.String(role.String()),
		},
	}

	vars := map[string]interface{}{
		"workspaceId": graphql.ID(id),
		"users":       users,
	}

	return r.client.Mutate(ctx, &m, vars)
}

func (r *workspaceRepo) RemoveUser(ctx context.Context, workspaceID string, userID string) error {
	var m removeUserFromWorkspaceMutation
	vars := map[string]interface{}{
		"workspaceId": graphql.ID(workspaceID),
		"userId":      graphql.ID(userID),
	}

	return r.client.Mutate(ctx, &m, vars)
}

func (r *workspaceRepo) TransferOwnership(ctx context.Context, workspaceID string, newOwnerID string) (*workspace.Workspace, error) {
	var m transferWorkspaceOwnershipMutation
	vars := map[string]interface{}{
		"workspaceId": graphql.ID(workspaceID),
		"newOwnerId":  graphql.ID(newOwnerID),
	}

	if err := r.client.Mutate(ctx, &m, vars); err != nil {
		return nil, err
	}

	return gqlmodel.ToWorkspace(ctx, m.TransferWorkspaceOwnership.Workspace)
}
