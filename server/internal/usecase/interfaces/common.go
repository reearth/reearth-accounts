package interfaces

import (
	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/rerror"
)

var (
	ErrOperationDenied error = rerror.NewE(i18n.T("operation denied"))
	ErrInvalidOperator error = rerror.NewE(i18n.T("invalid operator"))
)

type Container struct {
	User        User
	Workspace   Workspace
	Cerbos      Cerbos
	Role        Role
	Permittable Permittable
}
