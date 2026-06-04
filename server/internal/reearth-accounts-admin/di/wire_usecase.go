//go:build wireinject
// +build wireinject

package di

import (
	"github.com/goforj/wire"
	"github.com/reearth/reearth-accounts/server/internal/reearth-accounts-admin/usecase/authz"
	"github.com/reearth/reearth-accounts/server/internal/reearth-accounts-admin/usecase/useruc"
)

// usecaseWire provides the admin authorization checker and the usecases
// (one struct per action).
var usecaseWire = wire.NewSet(
	authz.NewChecker,
	useruc.NewListUsersUseCase,
)