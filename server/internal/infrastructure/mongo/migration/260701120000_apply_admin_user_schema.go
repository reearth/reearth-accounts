package migration

import "context"

// ApplyAdminUserSchema creates the adminuser collection with its JSON schema
// validator (or updates the validator if the collection already exists).
func ApplyAdminUserSchema(ctx context.Context, c DBClient) error {
	return ApplyCollectionSchemas(ctx, []string{"adminuser"}, c)
}
