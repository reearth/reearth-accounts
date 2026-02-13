package gqlmodel

import (
	"github.com/hasura/go-graphql-client"
)

type UpdateUserOfWorkspaceInput struct {
	WorkspaceID graphql.ID     `json:"workspaceId"`
	UserID      graphql.ID     `json:"userId"`
	Role        graphql.String `json:"role"`
}
