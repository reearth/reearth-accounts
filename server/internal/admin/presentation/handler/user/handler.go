// Package user holds the admin user HTTP handlers. Each action lives in its own
// file (handler_<verb>_<noun>.go) and is backed by a dedicated usecase struct.
package user

import (
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/useruc"
)

// Handler aggregates the user-related admin usecases.
type Handler struct {
	getUC           *useruc.GetUserUseCase
	getWorkspacesUC *useruc.GetUserWorkspacesUseCase
	listUC          *useruc.ListUsersUseCase
}

// NewHandler is a Wire provider for the user Handler.
func NewHandler(getUC *useruc.GetUserUseCase, getWorkspacesUC *useruc.GetUserWorkspacesUseCase, listUC *useruc.ListUsersUseCase) *Handler {
	return &Handler{getUC: getUC, getWorkspacesUC: getWorkspacesUC, listUC: listUC}
}
