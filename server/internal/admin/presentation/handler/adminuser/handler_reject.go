package adminuser

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/admin/presentation/internal"
	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
)

// RejectAdminUser godoc
//
//	@Summary		Reject or revoke an admin user
//	@Description	Rejects a pending admin user or revokes an approved one. Cannot reject your own account, and the last approved admin cannot be rejected.
//	@Tags			admin-users
//	@Produce		json
//	@Param			id	path		string	true	"Admin user ID"
//	@Success		200	{object}	AdminUserResponse
//	@Failure		400	{object}	internal.ErrorResponse	"invalid id / cannot modify self / last approved admin"
//	@Failure		401	{object}	internal.ErrorResponse	"unauthorized"
//	@Failure		403	{object}	internal.ErrorResponse	"not approved"
//	@Failure		404	{object}	internal.ErrorResponse	"admin user not found"
//	@Router			/admin-users/{id}/reject [post]
func (h *Handler) RejectAdminUser(c echo.Context) error {
	operator, err := internal.GetAdminUser(c)
	if err != nil {
		return err
	}
	targetID, err := adminuser.IDFrom(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}

	u, err := h.reject.Execute(c.Request().Context(), operator.ID(), targetID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, newAdminUserResponse(u))
}
