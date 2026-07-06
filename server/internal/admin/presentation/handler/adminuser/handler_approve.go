package adminuser

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/admin/presentation/internal"
	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
)

// ApproveAdminUser godoc
//
//	@Summary		Approve an admin user
//	@Description	Approves a pending or rejected admin user, recording the current admin as approver. Idempotent for already-approved users (the original approver/timestamp is kept). Cannot approve your own account.
//	@Tags			admin-users
//	@Produce		json
//	@Param			id	path		string	true	"Admin user ID"
//	@Success		200	{object}	AdminUserResponse
//	@Failure		400	{object}	internal.ErrorResponse	"invalid id / cannot modify self"
//	@Failure		401	{object}	internal.ErrorResponse	"unauthorized"
//	@Failure		403	{object}	internal.ErrorResponse	"not approved"
//	@Failure		404	{object}	internal.ErrorResponse	"not found"
//	@Router			/admin-users/{id}/approve [post]
func (h *Handler) ApproveAdminUser(c echo.Context) error {
	operator, err := internal.GetAdminUser(c)
	if err != nil {
		return err
	}
	targetID, err := adminuser.IDFrom(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}

	u, err := h.approve.Execute(c.Request().Context(), operator.ID(), targetID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, newAdminUserResponse(u))
}
