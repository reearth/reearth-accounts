package role

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/id"
)

//go:generate mockgen -source=./repo.go -destination=./mock_role.go -package role
type Repo interface {
	FindAll(context.Context) (List, error)
	FindByID(context.Context, id.RoleID) (*Role, error)
	FindByIDs(context.Context, id.RoleIDList) (List, error)
	FindByName(ctx context.Context, name string) (*Role, error)
	Save(context.Context, Role) error
	Remove(context.Context, id.RoleID) error
}
