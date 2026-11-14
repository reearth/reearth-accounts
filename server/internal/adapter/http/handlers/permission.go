package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/adapter/http/httpmodel"
	httpinternal "github.com/reearth/reearth-accounts/server/internal/adapter/http/internal"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/samber/lo"
)

type PermissionHandler struct{}

func NewPermissionHandler() *PermissionHandler { return &PermissionHandler{} }

// Check godoc
// @Tags Permission
// @Summary Check a permission for the current user
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body httpmodel.CheckPermissionRequest true "service/resource/action"
// @Success 200 {object} httpmodel.CheckPermissionResponse
// @Failure 401 {object} internal.ErrorResponse
// @Router /api/permissions/check [post]
//
// The interactor checks a user's permission, so a service caller gets 401.
func (h *PermissionHandler) Check(c echo.Context) error {
	ctx := c.Request().Context()
	req := &httpmodel.CheckPermissionRequest{}
	if err := httpinternal.BindValidate(c, req); err != nil {
		return err
	}
	u, err := httpinternal.RequireUser(c)
	if err != nil {
		return err
	}
	res, err := httpinternal.Usecases(c).Cerbos.CheckPermission(ctx, u.ID(), interfaces.CheckPermissionParam{
		Service:        req.Service,
		Resource:       req.Resource,
		Action:         req.Action,
		WorkspaceAlias: lo.FromPtr(req.WorkspaceAlias),
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpmodel.CheckPermissionResponse{Allowed: res.Allowed})
}
