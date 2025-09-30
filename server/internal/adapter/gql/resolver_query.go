package gql

import (
	"context"

	"github.com/reearth/reearth-accounts/internal/adapter/gql/gqlmodel"
)

func (r *Resolver) Query() QueryResolver {
	return &queryResolver{r}
}

type queryResolver struct{ *Resolver }

func (r *queryResolver) Me(ctx context.Context) (*gqlmodel.Me, error) {
	u := getUser(ctx)
	if u == nil {
		return nil, nil
	}

	if u.Metadata().PhotoURL() != "" {
		signedURL, err := r.Storage.GetSignedURL(ctx, u.Metadata().PhotoURL())
		if err != nil {
			return nil, err
		}
		u.Metadata().SetPhotoURL(signedURL)
	}

	return gqlmodel.ToMe(u), nil
}

func (r *queryResolver) Node(ctx context.Context, i gqlmodel.ID, typeArg gqlmodel.NodeType) (gqlmodel.Node, error) {
	dataloaders := dataloaders(ctx)
	switch typeArg {
	case gqlmodel.NodeTypeUser:
		result, err := dataloaders.User.Load(i)
		if result == nil {
			return nil, nil
		}
		return result, err
	case gqlmodel.NodeTypeWorkspace:
		result, err := dataloaders.Workspace.Load(i)
		if result == nil {
			return nil, nil
		}
		return result, err
	}
	return nil, nil
}

func (r *queryResolver) Nodes(ctx context.Context, ids []gqlmodel.ID, typeArg gqlmodel.NodeType) ([]gqlmodel.Node, error) {
	dataloaders := dataloaders(ctx)
	switch typeArg {
	case gqlmodel.NodeTypeUser:
		data, err := dataloaders.User.LoadAll(ids)
		if len(err) > 0 && err[0] != nil {
			return nil, err[0]
		}
		nodes := make([]gqlmodel.Node, len(data))
		for i := range data {
			nodes[i] = data[i]
		}
		return nodes, nil
	case gqlmodel.NodeTypeWorkspace:
		data, err := dataloaders.Workspace.LoadAll(ids)
		if len(err) > 0 && err[0] != nil {
			return nil, err[0]
		}
		nodes := make([]gqlmodel.Node, len(data))
		for i := range data {
			nodes[i] = data[i]
		}
		return nodes, nil
	}
	return nil, nil
}

func (r *queryResolver) UserByNameOrEmail(ctx context.Context, nameOrEmail string) (*gqlmodel.User, error) {
	return loaders(ctx).User.SearchUser(ctx, nameOrEmail)
}

// Temporary stub implementation to satisfy gqlgen after migrating GraphQL files from reearthx/account.
// This resolver was added to avoid compile-time errors.
// Will be implemented if needed, or removed if unused after migration.
func (r *queryResolver) SearchUser(ctx context.Context, nameOrEmail string) ([]*gqlmodel.User, error) {
	return nil, nil
}
