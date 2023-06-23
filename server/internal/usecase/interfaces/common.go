package interfaces

import (
	"github.com/reearth/reearthx/account/accountusecase/accountinterfaces"
)

type Container struct {
	User      accountinterfaces.User
	Workspace accountinterfaces.Workspace
}
