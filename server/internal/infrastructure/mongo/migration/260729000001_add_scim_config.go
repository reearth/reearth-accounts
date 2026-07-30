package migration

import "context"

// AddScimConfig updates the workspaces collection JSON schema validator to pick
// up the new optional scimconfig embedded document and the externalid field on
// member sub-documents.
func AddScimConfig(ctx context.Context, c DBClient) error {
	return ApplyCollectionSchemas(ctx, []string{"workspaces"}, c)
}
