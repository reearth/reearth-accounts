//go:generate go run go.uber.org/mock/mockgen -source=repo.go -destination=mockrepo/mockrepo.go -package=mockrepo -mock_names=Repo=MockRepo

package cerbos

import (
	"context"

	"github.com/hasura/go-graphql-client"
	"github.com/reearth/reearth-accounts/server/pkg/gqlclient/gqlerror"
)

type cerbosRepo struct {
	client *graphql.Client
}

// CheckPermissionParam represents the parameters for checking a permission
type CheckPermissionParam struct {
	Service        string
	Resource       string
	Action         string
	WorkspaceAlias *string
}

// CheckPermissionResult represents the result of a permission check
type CheckPermissionResult struct {
	Allowed bool
}

// Repo provides methods for interacting with the Cerbos permission system via GraphQL
type Repo interface {
	// CheckPermission checks if the current authenticated user has permission to perform an action on a resource
	CheckPermission(ctx context.Context, param CheckPermissionParam) (*CheckPermissionResult, error)
}

// NewRepo creates a new Cerbos repository
func NewRepo(gql *graphql.Client) Repo {
	return &cerbosRepo{client: gql}
}

// CheckPermission checks if the authenticated user has permission to perform the specified action
func (r *cerbosRepo) CheckPermission(ctx context.Context, param CheckPermissionParam) (*CheckPermissionResult, error) {
	var q checkPermissionQuery

	input := CheckPermissionInput{
		Service:  graphql.String(param.Service),
		Resource: graphql.String(param.Resource),
		Action:   graphql.String(param.Action),
	}

	if param.WorkspaceAlias != nil {
		ws := graphql.String(*param.WorkspaceAlias)
		input.WorkspaceAlias = &ws
	}

	vars := map[string]interface{}{
		"input": input,
	}

	if err := r.client.Query(ctx, &q, vars); err != nil {
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	return &CheckPermissionResult{
		Allowed: bool(q.CheckPermission.Allowed),
	}, nil
}
