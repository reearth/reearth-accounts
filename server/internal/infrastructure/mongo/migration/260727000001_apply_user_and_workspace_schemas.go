package migration

import "context"

func ApplyUserAndWorkspaceSchemas(ctx context.Context, c DBClient) error {
	return ApplyCollectionSchemas(ctx, []string{"users", "workspaces"}, c)
}
