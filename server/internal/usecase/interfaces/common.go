package interfaces

import (
	"errors"

	"github.com/reearth/reearthx/account/accountusecase/accountinterfaces"
)

var (
	ErrOperationDenied = errors.New("operation denied")
)

type Container struct {
	User      accountinterfaces.User
	Workspace accountinterfaces.Workspace
}
