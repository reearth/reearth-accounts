package workspace

import (
	"context"

	"github.com/hasura/go-graphql-client"
	"github.com/reearth/reearth-accounts/server/pkg/gqlclient/gqlmodel"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
)

type workspaceRepo struct {
	client *graphql.Client
}

type WorkspaceRepo interface {
	FindByUser(ctx context.Context, userID string) (workspace.List, error)
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
		return nil, err
	}

	return gqlmodel.ToWorkspaces(q.FindByUser), nil
}
