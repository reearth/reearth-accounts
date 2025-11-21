package interfaces

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/usecase"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/usecasex"
)

type User interface {
	Fetch(context.Context, []id.UserID, *usecase.Operator) ([]*user.User, error)
	FindByID(context.Context, id.UserID, *usecase.Operator) (*user.User, error)
	FindByIDs(context.Context, id.UserIDList, *usecase.Operator) ([]*user.User, error)
	FindByEmail(context.Context, string) (*user.User, error)
	FindByName(context.Context, string) (*user.User, error)
	FindByNameOrEmail(context.Context, string) (*user.User, error)
	FindByVerification(context.Context, string) (*user.User, error)
	Create(context.Context, *user.User, *usecase.Operator) (*user.User, error)
	Update(context.Context, *user.User, *usecase.Operator) (*user.User, error)
	UpdateProfile(context.Context, id.UserID, string, string, *usecase.Operator) (*user.User, error)
	Remove(context.Context, id.UserID, *usecase.Operator) error
	SearchUser(context.Context, id.WorkspaceIDList, string, *usecasex.Pagination) ([]*user.User, *usecasex.PageInfo, error)
}
