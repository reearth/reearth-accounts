package interfaces

import (
	"context"

	"github.com/reearth/reearthx/account/accountdomain/user"
)

type CheckPermissionParam struct {
	Service  string
	Resource string
	Action   string
}

type CheckPermissionResult struct {
	Allowed bool
}

type Cerbos interface {
	CheckPermission(ctx context.Context, param CheckPermissionParam, user *user.User) (*CheckPermissionResult, error)
}
