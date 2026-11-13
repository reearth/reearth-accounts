package gqlmodel

import (
	"context"
	"errors"

	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/pkg/applog"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/sourcegraph/conc/pool"
)

func ToWorkspace(
	ctx context.Context,
	t *workspace.Workspace,
	exists map[user.ID]struct{},
	photos adapter.PhotoSigner,
) (*Workspace, error) {
	if t == nil {
		applog.Error(ctx, "workspace is nil")
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
		Description: t.Metadata().Description(),
		Website:     t.Metadata().Website(),
		Location:    t.Metadata().Location(),
	}
	if viewer := adapter.User(ctx); viewer != nil && t.Members().HasUser(viewer.ID()) {
		metadata.BillingEmail = t.Metadata().BillingEmail()
	}

	if t.Metadata() != nil {
		signedURL, sErr := photos.Sign(ctx, t.Metadata().PhotoURL())
		if sErr != nil {
			applog.Error(ctx, "gql: failed to sign photo url for workspace", applog.WorkspaceID(t.ID()), applog.Err(sErr))
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
	photos adapter.PhotoSigner,
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
			converted, err := ToWorkspace(ctx, w, exists, photos)
			if err != nil {
				applog.Error(ctx, "failed to convert workspace", applog.Err(err))
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
