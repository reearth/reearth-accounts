package policy

import (
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/policyuc"
)

type Handler struct {
	create *policyuc.CreateNoticeUseCase
	list   *policyuc.ListNoticesUseCase
}

func NewHandler(create *policyuc.CreateNoticeUseCase, list *policyuc.ListNoticesUseCase) *Handler {
	return &Handler{create: create, list: list}
}
