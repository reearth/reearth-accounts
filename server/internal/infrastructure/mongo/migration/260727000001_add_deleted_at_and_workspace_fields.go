package migration

import "context"

func AddDeletedAtAndWorkspaceFields(ctx context.Context, c DBClient) error {
	return ApplyCollectionSchemas(ctx, []string{"users", "workspaces"}, c)
}
