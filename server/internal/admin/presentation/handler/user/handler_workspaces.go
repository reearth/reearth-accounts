package user

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/pkg/id"
)

// GetUserWorkspaces godoc
//
//	@Summary		List a user's workspaces
//	@Description	Returns the workspaces the user belongs to, with the user's role in each. A user in no workspace returns an empty list.
//	@Tags			users
//	@Produce		json
//	@Param			id	path		string	true	"User ID"
//	@Success		200	{array}		UserWorkspaceResponse
//	@Failure		400	{object}	internal.ErrorResponse	"invalid id"
//	@Failure		401	{object}	internal.ErrorResponse	"unauthorized"
//	@Failure		403	{object}	internal.ErrorResponse	"not approved"
//	@Router			/users/{id}/workspaces [get]
func (h *Handler) GetUserWorkspaces(c echo.Context) error {
	uid, err := id.UserIDFrom(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}

	list, err := h.getWorkspacesUC.Execute(c.Request().Context(), uid)
	if err != nil {
		return err
	}

	res := make([]UserWorkspaceResponse, 0, len(list))
	for _, ws := range list {
		item := UserWorkspaceResponse{
			ID:       ws.ID().String(),
			Name:     ws.Name(),
			Alias:    ws.Alias(),
			Personal: ws.IsPersonal(),
		}
		if m := ws.Members(); m != nil {
			item.Role = string(m.UserRole(uid))
		}
		res = append(res, item)
	}
	return c.JSON(http.StatusOK, res)
}
