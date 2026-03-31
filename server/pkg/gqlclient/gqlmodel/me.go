package gqlmodel

import (
	"time"

	"github.com/hasura/go-graphql-client"
)

type Me struct {
	ID             graphql.ID       `json:"id" graphql:"id"`
	Name           graphql.String   `json:"name" graphql:"name"`
	Alias          graphql.String   `json:"alias" graphql:"alias"`
	Email          graphql.String   `json:"email" graphql:"email"`
	LatestLogoutAt *time.Time       `json:"latestLogoutAt" graphql:"latestLogoutAt"`
	Metadata       UserMetadata     `json:"metadata" graphql:"metadata"`
	Host           graphql.String   `json:"host" graphql:"host"`
	MyWorkspaceID  graphql.ID       `json:"myWorkspaceId" graphql:"myWorkspaceId"`
	Auths          []graphql.String `json:"auths" graphql:"auths"`
}
