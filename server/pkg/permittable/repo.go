package permittable

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/user"
)

//go:generate mockgen -source=./repo.go -destination=./mock_permittable.go -package permittable
type Repo interface {
	FindByUserID(context.Context, user.ID) (*Permittable, error)
	FindByUserIDs(context.Context, user.IDList) (List, error)
	// FindByUserIDsForUpdate is FindByUserIDs that, on Postgres, also row-locks
	// the found permittables until the surrounding transaction ends, taking the
	// locks in user ID order so two callers locking overlapping users cannot
	// deadlock. It only serializes callers that also use it; a plain
	// FindByUserID followed by Save is not blocked. A user without a
	// permittable has no row to lock. Mongo and memory take no lock.
	FindByUserIDsForUpdate(context.Context, user.IDList) (List, error)
	FindByRoleID(context.Context, id.RoleID) (List, error)
	Save(context.Context, Permittable) error
	SaveMany(context.Context, List) error
}
