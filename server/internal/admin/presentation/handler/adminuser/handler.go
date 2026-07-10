// Package adminuser implements the admin-user management endpoints, all behind
// the RequireApproved middleware.
package adminuser

import (
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/adminuseruc"
)

// Handler serves the /admin-users endpoints.
type Handler struct {
	list    *adminuseruc.ListAdminUsersUseCase
	approve *adminuseruc.ApproveAdminUserUseCase
	reject  *adminuseruc.RejectAdminUserUseCase
	setRole *adminuseruc.SetRoleUseCase
}

// NewHandler is a Wire provider for the admin-user Handler.
func NewHandler(
	list *adminuseruc.ListAdminUsersUseCase,
	approve *adminuseruc.ApproveAdminUserUseCase,
	reject *adminuseruc.RejectAdminUserUseCase,
	setRole *adminuseruc.SetRoleUseCase,
) *Handler {
	return &Handler{list: list, approve: approve, reject: reject, setRole: setRole}
}
