package gqlmodel

import (
	"context"
	"errors"

	"github.com/labstack/gommon/log"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
)

func ToWorkspace(
	t *workspace.Workspace,
	exists map[user.ID]struct{},
	storage gateway.Storage,
) (*Workspace, error) {
	if t == nil {
		log.Error("workspace is nil")
		return nil, errors.New("workspace is nil")
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
	}

	if t.Metadata() != nil && t.Metadata().PhotoURL() != "" {
		signedURL, sErr := storage.GetSignedURL(context.Background(), t.Metadata().PhotoURL())
		if sErr != nil {
			return nil, sErr
		}
		t.Metadata().SetPhotoURL(signedURL)
	}

	return &Workspace{
		ID:       IDFrom(t.ID()),
		Name:     t.Name(),
		Alias:    t.Alias(),
		Personal: t.IsPersonal(),
		Members:  members,
		Metadata: &metadata,
	}, nil
}

func ToWorkspaces(
	ws workspace.List,
	exists map[user.ID]struct{},
	storage gateway.Storage,
) []*Workspace {
	if ws == nil {
		return nil
	}

	workspaces := make([]*Workspace, 0, len(ws))
	for _, w := range ws {
		converted, err := ToWorkspace(w, exists, storage)
		if err != nil {
			log.Errorf("failed to convert workspace: %s", err.Error())
			continue
		}
		workspaces = append(workspaces, converted)
	}
	return workspaces
}
