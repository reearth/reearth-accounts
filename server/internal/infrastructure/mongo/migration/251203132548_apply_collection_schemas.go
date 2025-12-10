package migration

import (
	"context"
)

// ApplyCollectionSchemas creates collections with JSON schema validators if they don't exist,
// or updates existing collections with the schema validators.
// Schemas are read from embedded JSON files in the schema package.
func ApplyCollectionSchemas1(ctx context.Context, c DBClient) error {
	return ApplyCollectionSchemas(ctx, []string{"user", "workspace", "role", "permittable", "config"}, c)
}
