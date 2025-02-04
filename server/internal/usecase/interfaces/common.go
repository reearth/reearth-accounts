package interfaces

import (
	"errors"

	"github.com/reearth/reearthx/account/accountusecase/accountinterfaces"
)

var (
	ErrOperationDenied error = errors.New("operation denied")
)

type Container struct {
	User        accountinterfaces.User
	Workspace   accountinterfaces.Workspace
	Cerbos      Cerbos
	Role        Role
	Group       Group
	Permittable Permittable
}
