package gqlmodel

import "github.com/hasura/go-graphql-client"

type WorkspaceUser struct {
	ID    graphql.ID     `graphql:"id"`
	Name  graphql.String `graphql:"name"`
	Email graphql.String `graphql:"email"`
}

type WorkspaceUserMember struct {
	UserID graphql.ID     `graphql:"userId"`
	Role   graphql.String `graphql:"role"`
	User   WorkspaceUser  `graphql:"user"`
}

type WorkspaceInviter struct {
	ID   graphql.ID     `graphql:"id"`
	Name graphql.String `graphql:"name"`
}

// WorkspaceIntegrationMember（Integration 参加者情報）
type WorkspaceIntegrationMember struct {
	IntegrationID graphql.ID        `graphql:"integrationId"`
	Role          graphql.String    `graphql:"role"`
	Active        graphql.Boolean   `graphql:"active"`
	InvitedByID   graphql.ID        `graphql:"invitedById"`
	InvitedBy     *WorkspaceInviter `graphql:"invitedBy"`
}

type WorkspaceMember struct {
	Typename          graphql.String             `graphql:"__typename"`
	UserMember        WorkspaceUserMember        `graphql:"... on WorkspaceUserMember"`
	IntegrationMember WorkspaceIntegrationMember `graphql:"... on WorkspaceIntegrationMember"`
}
