//go:build wireinject
// +build wireinject

package di

import (
	"github.com/goforj/wire"
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/adminuseruc"
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/authuc"
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/authz"
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/useruc"
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/workspaceuc"
)

// usecaseWire provides the admin authorization checker and the usecases
// (one struct per action).
var usecaseWire = wire.NewSet(
	authz.NewChecker,
	useruc.NewListUsersUseCase,

	// session auth dependencies + usecases
	provideGoogleVerifier,
	provideSessionManager,
	provideGoogleSignInOptions,
	authuc.NewGoogleSignInUseCase,
	authuc.NewGetMeUseCase,

	// admin-user management usecases
	adminuseruc.NewListAdminUsersUseCase,
	adminuseruc.NewApproveAdminUserUseCase,
	adminuseruc.NewRejectAdminUserUseCase,

	// cross-tenant workspace usecases
	workspaceuc.NewListWorkspacesUseCase,
)
