package gql

import (
	"context"

	"github.com/reearth/reearth-accounts/server/internal/adapter/gql/gqlmodel"
)

func (r *Resolver) WorkspaceUserMember() WorkspaceUserMemberResolver {
	return &workspaceUserMemberResolver{r}
}

type workspaceUserMemberResolver struct{ *Resolver }

func (w workspaceUserMemberResolver) User(ctx context.Context, obj *gqlmodel.WorkspaceUserMember) (*gqlmodel.User, error) {
	return dataloaders(ctx).User.Load(obj.UserID)
}

// Temporary stub implementation to satisfy gqlgen after migrating GraphQL files from reearthx/account.
// This resolver was added to avoid compile-time errors.
// Will be implemented if needed, or removed if unused after migration.
func (r *queryResolver) FindByUser(ctx context.Context, userId gqlmodel.ID) ([]*gqlmodel.Workspace, error) {
	return nil, nil
}
