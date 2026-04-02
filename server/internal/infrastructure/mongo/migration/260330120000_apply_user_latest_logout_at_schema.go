package migration

import (
	"context"
)

func ApplyUserLatestLogoutAtSchema(ctx context.Context, c DBClient) error {
	return ApplyCollectionSchemas(ctx, []string{"user"}, c)
}
