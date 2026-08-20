package migration

import "context"

// ApplyUserAndWorkspaceSchemas re-applies the JSON schema validators for the
// user and workspace collections, which gained createdat/createdby/deletedat.
// Collection names must match both the embedded schema file names in
// internal/infrastructure/mongo/schema and the names the repositories use.
func ApplyUserAndWorkspaceSchemas(ctx context.Context, c DBClient) error {
	return ApplyCollectionSchemas(ctx, []string{"user", "workspace"}, c)
}
