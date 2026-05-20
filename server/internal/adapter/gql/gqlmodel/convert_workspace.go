package gqlmodel

import (
	"context"
	"errors"

	"github.com/labstack/gommon/log"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/sourcegraph/conc/pool"
)

func ToWorkspace(
	ctx context.Context,
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

	log.Infof("[ToWorkspace] convert workspace photo url: %s", t.Metadata().PhotoURL())
	if t.Metadata() != nil && t.Metadata().PhotoURL() != "" {
		signedURL, sErr := storage.GetSignedURL(ctx, t.Metadata().PhotoURL())
		log.Infof("[ToWorkspace] get signed url: %s", signedURL)
		if sErr != nil {
			log.Errorf("[ToWorkspace] failed to get signed url: %s, workspace id: %s", sErr.Error(), t.ID())
		}
		metadata.PhotoURL = signedURL
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
	ctx context.Context,
	ws workspace.List,
	exists map[user.ID]struct{},
	storage gateway.Storage,
) []*Workspace {
	if ws == nil {
		return nil
	}

	const maxConcurrency = 10
	results := make([]*Workspace, len(ws))

	p := pool.New().WithMaxGoroutines(maxConcurrency)

	for i, w := range ws {
		i, w := i, w
		p.Go(func() {
			converted, err := ToWorkspace(ctx, w, exists, storage)
			if err != nil {
				log.Errorf("failed to convert workspace: %s", err.Error())
				return
			}
			results[i] = converted
		})
	}

	p.Wait()

	workspaces := make([]*Workspace, 0, len(results))
	for _, w := range results {
		if w != nil {
			workspaces = append(workspaces, w)
		}
	}
	return workspaces
}
