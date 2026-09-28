package interfaces

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/user"
)

type Permittable interface {
	// FindByUserIDs batch-fetches global/workspace role bindings for a set of users,
	// e.g. to enrich a paginated user list without one query per row (N+1).
	FindByUserIDs(ctx context.Context, userIDs user.IDList) (permittable.List, error)
}
