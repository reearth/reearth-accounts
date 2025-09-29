package gqlmodel

import (
	"github.com/reearth/reearth-accounts/pkg/user"
	"github.com/reearth/reearth-accounts/pkg/workspace"
)

func ToWorkspace(t *workspace.Workspace, exists map[user.ID]struct{}) *Workspace {
	if t == nil {
		return nil
	}

	usersMap := t.Members().Users()
	integrationsMap := t.Members().Integrations()
	members := make([]WorkspaceMember, 0, len(usersMap)+len(integrationsMap))
	for u, m := range usersMap {
		if exists != nil {
			if _, ok := exists[u]; !ok {
				continue
			}
		}
		members = append(members, &WorkspaceUserMember{
			UserID: IDFrom(u),
			Role:   ToRole(m.Role),
		})
	}

	metadata := WorkspaceMetadata{
		Description:  t.Metadata().Description(),
		Website:      t.Metadata().Website(),
		Location:     t.Metadata().Location(),
		BillingEmail: t.Metadata().BillingEmail(),
		PhotoURL:     t.Metadata().PhotoURL(),
	}

	return &Workspace{
		ID:       IDFrom(t.ID()),
		Name:     t.Name(),
		Alias:    t.Alias(),
		Personal: t.IsPersonal(),
		Members:  members,
		Metadata: &metadata,
	}
}

func ToWorkspaces(ws workspace.List, exists map[user.ID]struct{}) []*Workspace {
	if ws == nil {
		return nil
	}

	workspaces := make([]*Workspace, 0, len(ws))
	for _, w := range ws {
		workspaces = append(workspaces, ToWorkspace(w, exists))
	}
	return workspaces
}
