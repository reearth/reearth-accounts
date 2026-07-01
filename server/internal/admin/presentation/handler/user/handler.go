// Package user holds the admin user HTTP handlers. Each action lives in its own
// file (handler_<verb>_<noun>.go) and is backed by a dedicated usecase struct.
package user

import (
	_ "github.com/reearth/reearth-accounts/server/internal/admin/presentation/internal" // for swagger ErrorResponse
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/useruc"
)

// Handler aggregates the user-related admin usecases.
type Handler struct {
	listUC *useruc.ListUsersUseCase
}

// NewHandler is a Wire provider for the user Handler.
func NewHandler(listUC *useruc.ListUsersUseCase) *Handler {
	return &Handler{listUC: listUC}
}
