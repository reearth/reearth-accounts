package cerbos

import "github.com/hasura/go-graphql-client"

type checkPermissionQuery struct {
	CheckPermission struct {
		Allowed graphql.Boolean `graphql:"allowed"`
	} `graphql:"checkPermission(input: $input)"`
}

type CheckPermissionInput struct {
	Service        graphql.String  `json:"service"`
	Resource       graphql.String  `json:"resource"`
	Action         graphql.String  `json:"action"`
	WorkspaceAlias *graphql.String `json:"workspaceAlias,omitempty"`
}
