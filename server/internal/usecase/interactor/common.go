package interactor

import (
	"github.com/reearth/reearth-account/internal/usecase/interfaces"
	"github.com/reearth/reearthx/account/accountusecase/accountgateway"
	"github.com/reearth/reearthx/account/accountusecase/accountinteractor"
	"github.com/reearth/reearthx/account/accountusecase/accountrepo"
)

type ContainerConfig struct {
	SignupSecret    string
	AuthSrvUIDomain string
}

func NewContainer(
	acr *accountrepo.Container,
	acg *accountgateway.Container,
	config ContainerConfig) interfaces.Container {
	return interfaces.Container{
		User:      accountinteractor.NewUser(acr, acg, config.SignupSecret, config.AuthSrvUIDomain),
		Workspace: accountinteractor.NewWorkspace(acr),
	}
}
