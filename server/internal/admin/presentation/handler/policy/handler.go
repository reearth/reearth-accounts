// Package policy serves the admin policy notice endpoints, behind the
// RequireApproved middleware.
package policy

import (
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/policyuc"
)

type Handler struct {
	create *policyuc.CreateNoticeUseCase
	list   *policyuc.ListNoticesUseCase
}

// NewHandler is a Wire provider for the policy Handler.
func NewHandler(create *policyuc.CreateNoticeUseCase, list *policyuc.ListNoticesUseCase) *Handler {
	return &Handler{create: create, list: list}
}
