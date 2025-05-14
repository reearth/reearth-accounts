package interfaces

import (
	"errors"

	"github.com/reearth/reearthx/account/accountusecase/accountinterfaces"
	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/rerror"
)

var (
	ErrOperationDeniedOld error = errors.New("operation denied")
	ErrOperationDenied    error = rerror.NewE(i18n.T("operation denied"))
	ErrInvalidOperator    error = rerror.NewE(i18n.T("invalid operator"))
)

type Container struct {
	User        accountinterfaces.User
	Workspace   accountinterfaces.Workspace
	Cerbos      Cerbos
	Role        Role
	Permittable Permittable
}
