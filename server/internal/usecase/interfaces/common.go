package interfaces

import (
	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/rerror"
)

var (
	ErrOperationDenied error = rerror.NewE(i18n.T("operation denied"))
	ErrInvalidOperator error = rerror.NewE(i18n.T("invalid operator"))
	ErrInvalidPhotoURL error = rerror.NewE(i18n.T("invalid photo URL"))
)

type Container struct {
	Cerbos    Cerbos
	Role      Role
	User      User
	Workspace Workspace
}
