package workspace

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/pkg/id"
)

// GetWorkspace godoc
//
//	@Summary		Get a workspace
//	@Description	Returns the detail of a single workspace by ID.
//	@Tags			workspaces
//	@Produce		json
//	@Param			id	path		string	true	"Workspace ID"
//	@Success		200	{object}	WorkspaceResponse
//	@Failure		400	{object}	internal.ErrorResponse	"invalid id"
//	@Failure		401	{object}	internal.ErrorResponse	"unauthorized"
//	@Failure		403	{object}	internal.ErrorResponse	"not approved"
//	@Failure		404	{object}	internal.ErrorResponse	"not found"
//	@Router			/workspaces/{id} [get]
func (h *Handler) GetWorkspace(c echo.Context) error {
	wid, err := id.WorkspaceIDFrom(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}

	ws, err := h.get.Execute(c.Request().Context(), wid)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, newWorkspaceResponse(ws))
}
