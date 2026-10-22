package workspace

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/admin/presentation/internal"
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/workspaceuc"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearthx/log"
)

// TransferWorkspaceOwnerRequest is the request body for transferring ownership.
type TransferWorkspaceOwnerRequest struct {
	NewOwnerUserID string `json:"newOwnerUserId" validate:"required"`
} // @name TransferWorkspaceOwnerRequest

// TransferWorkspaceOwner godoc
//
//	@Summary		Transfer a workspace's ownership
//	@Description	Makes an existing, enabled member of a team workspace its sole owner. Every other owner becomes a maintainer. A reader may be made owner. Role and permittable changes are committed atomically.
//	@Tags			workspaces
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string							true	"Workspace ID"
//	@Param			body	body		TransferWorkspaceOwnerRequest	true	"New owner"
//	@Success		200		{object}	TransferWorkspaceOwnerResponse
//	@Failure		400		{object}	internal.ErrorResponse	"invalid id / invalid request body"
//	@Failure		401		{object}	internal.ErrorResponse	"unauthorized"
//	@Failure		403		{object}	internal.ErrorResponse	"not approved / forbidden"
//	@Failure		404		{object}	internal.ErrorResponse	"workspace not found"
//	@Failure		409		{object}	internal.ErrorResponse	"already_owner"
//	@Failure		422		{object}	internal.ErrorResponse	"personal_workspace / target_not_member / target_disabled / target_user_not_found"
//	@Router			/workspaces/{id}/transfer-owner [post]
func (h *Handler) TransferWorkspaceOwner(c echo.Context) error {
	admin, err := internal.GetAdminUser(c)
	if err != nil {
		return err
	}
	wid, err := id.WorkspaceIDFrom(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}

	var body TransferWorkspaceOwnerRequest
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if body.NewOwnerUserID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "newOwnerUserId is required")
	}
	uid, err := id.UserIDFrom(body.NewOwnerUserID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid newOwnerUserId")
	}

	ctx := c.Request().Context()
	out, err := h.transferOwner.Execute(ctx, workspaceuc.TransferOwnerInput{
		WorkspaceID: wid,
		NewOwnerID:  uid,
	})
	if err != nil {
		return err
	}

	res := newTransferWorkspaceOwnerResponse(out)
	log.Infofc(ctx, "[admin] transfer workspace owner: admin=%s workspace=%s newOwner=%s demoted=%v",
		admin.ID(), res.Workspace.ID, res.NewOwner.UserID, res.DemotedUserIDs)

	return c.JSON(http.StatusOK, res)
}
