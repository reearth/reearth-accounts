package interfaces

import (
	"context"

	"github.com/reearth/reearth-accounts/pkg/user"
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
	CheckPermission(ctx context.Context, userId user.ID, param CheckPermissionParam) (*CheckPermissionResult, error)
}
