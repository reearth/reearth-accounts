package role

import (
	"context"
)

type Repo interface {
	FindAll(context.Context) (List, error)
	FindByID(context.Context, ID) (*Role, error)
	FindByIDs(context.Context, IDList) (List, error)
	Save(context.Context, Role) error
	Remove(context.Context, ID) error
}
