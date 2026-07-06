// Package adminuser implements the admin-user management endpoints (list /
// approve / reject), all behind the RequireApproved middleware.
package adminuser

import (
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/adminuseruc"
)

// Handler serves the /admin-users endpoints.
type Handler struct {
	list    *adminuseruc.ListAdminUsersUseCase
	approve *adminuseruc.ApproveAdminUserUseCase
	reject  *adminuseruc.RejectAdminUserUseCase
}

// NewHandler is a Wire provider for the admin-user Handler.
func NewHandler(
	list *adminuseruc.ListAdminUsersUseCase,
	approve *adminuseruc.ApproveAdminUserUseCase,
	reject *adminuseruc.RejectAdminUserUseCase,
) *Handler {
	return &Handler{list: list, approve: approve, reject: reject}
}
