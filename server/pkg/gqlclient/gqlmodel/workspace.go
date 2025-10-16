package gqlmodel

import (
	"github.com/hasura/go-graphql-client"
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

func ToWorkspace(w Workspace) *workspace.Workspace {
	id, err := workspace.IDFrom(string(w.ID))
	if err != nil {
		return nil
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
		Personal(w.Personal).
		MustBuild()
}

func ToWorkspaces(gqlWorkspaces []Workspace) workspace.List {
	workspaces := make(workspace.List, 0, len(gqlWorkspaces))
	for _, w := range gqlWorkspaces {
		if ws := ToWorkspace(w); ws != nil {
			workspaces = append(workspaces, ws)
		}
	}
	return workspaces
}
