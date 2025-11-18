package gqlmodel

import (
	"context"

	"github.com/hasura/go-graphql-client"
	"github.com/labstack/gommon/log"
	"github.com/reearth/reearth-accounts/server/pkg/gqlclient/gqlerror"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
)

type Workspace struct {
	ID       graphql.ID        `json:"id" graphql:"id"`
	Name     graphql.String    `json:"name" graphql:"name"`
	Alias    graphql.String    `json:"alias" graphql:"alias"`
	Members  []WorkspaceMember `graphql:"members"`
	Metadata WorkspaceMetadata `json:"metadata" graphql:"metadata"`
	Personal bool              `json:"personal" graphql:"personal"`
}

func ToWorkspace(ctx context.Context, w Workspace) (*workspace.Workspace, error) {
	id, err := workspace.IDFrom(string(w.ID))
	if err != nil {
		log.Errorf("[ToWorkspace] failed to convert workspace id: %s", w.ID)
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	members := make(map[workspace.UserID]workspace.Member)
	for _, m := range w.Members {
		userID, err := workspace.UserIDFrom(string(m.UserMember.UserID))
		if err != nil {
			log.Errorf("[ToWorkspace] failed to convert user id: %s", m.UserMember.UserID)
			return nil, gqlerror.ReturnAccountsError(ctx, err)
		}
		members[userID] = workspace.Member{
			Role: workspace.Role(m.UserMember.Role),
		}
	}

	return workspace.New().
		ID(id).
		Name(string(w.Name)).
		Alias(string(w.Alias)).
		Metadata(workspace.MetadataFrom(
			string(w.Metadata.Description),
			string(w.Metadata.Website),
			string(w.Metadata.Location),
			string(w.Metadata.BillingEmail),
			string(w.Metadata.PhotoURL),
		)).
		Members(members).
		Personal(w.Personal).
		MustBuild(), nil
}

func ToWorkspaces(ctx context.Context, gqlWorkspaces []Workspace) workspace.List {
	workspaces := make(workspace.List, 0, len(gqlWorkspaces))
	for _, w := range gqlWorkspaces {
		if ws, err := ToWorkspace(ctx, w); err != nil {
			log.Errorf("failed to convert workspace: %s", err.Error())
			continue
		} else if ws != nil {
			workspaces = append(workspaces, ws)
		}
	}
	return workspaces
}
