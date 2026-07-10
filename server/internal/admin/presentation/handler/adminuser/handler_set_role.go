package adminuser

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/admin/presentation/internal"
	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
)

// SetAdminUserRoleRequest is the request body for assigning a role.
type SetAdminUserRoleRequest struct {
	Role string `json:"role"`
} // @name SetAdminUserRoleRequest

// SetAdminUserRole godoc
//
//	@Summary		Assign a role to an admin user
//	@Description	Assigns a role (e.g. system_admin, viewer) to the target admin user. Changing your own role is allowed, but the last system_admin cannot be demoted (the system must never reach zero system_admins).
//	@Tags			admin-users
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"Admin user ID"
//	@Param			body	body		SetAdminUserRoleRequest	true	"Role to assign"
//	@Success		200		{object}	AdminUserResponse
//	@Failure		400		{object}	internal.ErrorResponse	"invalid id / invalid role"
//	@Failure		401		{object}	internal.ErrorResponse	"unauthorized"
//	@Failure		403		{object}	internal.ErrorResponse	"forbidden / cannot demote the last system admin"
//	@Failure		404		{object}	internal.ErrorResponse	"not found"
//	@Router			/admin-users/{id}/roles [put]
func (h *Handler) SetAdminUserRole(c echo.Context) error {
	operator, err := internal.GetAdminUser(c)
	if err != nil {
		return err
	}
	targetID, err := adminuser.IDFrom(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}

	var body SetAdminUserRoleRequest
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid role")
	}
	if body.Role == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid role")
	}
	role, err := adminuser.RoleFrom(body.Role)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid role")
	}

	u, err := h.setRole.Execute(c.Request().Context(), operator.ID(), targetID, role)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, newAdminUserResponse(u))
}
