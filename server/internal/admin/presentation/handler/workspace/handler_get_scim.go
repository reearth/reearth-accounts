package workspace

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/pkg/id"
)

// scimStateResponse is the JSON body for GET and PATCH /workspaces/:id/scim.
type scimStateResponse struct {
	ScimEnabled bool `json:"scim_enabled"`
}

// GetScimState godoc
//
//	@Summary		Get SCIM availability for a workspace
//	@Description	Returns whether the admin has enabled SCIM provisioning for the workspace (scim_available: true means a ScimConfig exists).
//	@Tags			workspaces
//	@Produce		json
//	@Param			id	path		string	true	"Workspace ID"
//	@Success		200	{object}	scimStateResponse
//	@Failure		400	{object}	internal.ErrorResponse	"invalid workspace id"
//	@Failure		401	{object}	internal.ErrorResponse	"unauthorized"
//	@Failure		403	{object}	internal.ErrorResponse	"not approved or forbidden"
//	@Failure		404	{object}	internal.ErrorResponse	"workspace not found"
//	@Router			/workspaces/{id}/scim [get]
func (h *Handler) GetScimState(c echo.Context) error {
	wid, err := id.WorkspaceIDFrom(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid workspace id")
	}
	available, err := h.getScim.Execute(c.Request().Context(), wid)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, scimStateResponse{ScimEnabled: available})
}