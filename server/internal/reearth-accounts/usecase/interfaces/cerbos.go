package interfaces

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
)

// RoleSelf is a special role that represents the user themselves
// Deprecated: Use role.RoleSelf instead
var RoleSelf = role.RoleSelf.String()

type CheckPermissionParam struct {
	Service        string
	Resource       string
	Action         string
	WorkspaceAlias string
}

type CheckPermissionResult struct {
	Allowed bool
}

type Cerbos interface {
	CheckPermission(ctx context.Context, userId user.ID, param CheckPermissionParam) (*CheckPermissionResult, error)
}
