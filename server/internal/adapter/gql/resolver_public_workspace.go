package gql

import (
	"context"
	"errors"
	"sort"

	"github.com/reearth/reearth-accounts/server/internal/adapter/gql/gqlmodel"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/rerror"
)

var errPublicWorkspaceKey = errors.New("publicWorkspace needs exactly one of id or alias")

// PublicWorkspace serves anonymous callers (ADR-0003 decision 2). It returns
// members' public profile only, and skips integrations and deleted users.
func (r *queryResolver) PublicWorkspace(ctx context.Context, wid *gqlmodel.ID, alias *string) (*gqlmodel.PublicWorkspace, error) {
	if (wid == nil) == (alias == nil) {
		return nil, errPublicWorkspaceKey
	}

	w, err := fetchPublicWorkspace(ctx, wid, alias)
	if errors.Is(err, rerror.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	uids := w.Members().UserIDs()
	users, err := usecases(ctx).User.FetchByID(ctx, uids)
	if err != nil {
		return nil, err
	}

	members := make([]*gqlmodel.PublicWorkspaceMember, 0, len(users))
	for _, u := range users {
		if u == nil {
			continue
		}
		members = append(members, &gqlmodel.PublicWorkspaceMember{
			Name:     u.Name(),
			Alias:    u.Alias(),
			PhotoURL: photoURL(u),
		})
	}
	sort.Slice(members, func(i, j int) bool { return members[i].Alias < members[j].Alias })

	return &gqlmodel.PublicWorkspace{
		ID:      gqlmodel.IDFrom(w.ID()),
		Name:    w.Name(),
		Alias:   w.Alias(),
		Members: members,
	}, nil
}

func fetchPublicWorkspace(ctx context.Context, wid *gqlmodel.ID, alias *string) (*workspace.Workspace, error) {
	if alias != nil {
		return usecases(ctx).Workspace.FetchByAlias(ctx, *alias, getOperator(ctx))
	}
	i, err := gqlmodel.ToID[id.Workspace](*wid)
	if err != nil {
		return nil, rerror.ErrNotFound
	}
	return usecases(ctx).Workspace.FetchByID(ctx, i, getOperator(ctx))
}

func photoURL(u *user.User) string {
	if u.Metadata() == nil {
		return ""
	}
	return u.Metadata().PhotoURL()
}
