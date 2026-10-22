package workspace

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/pkg/id"
)

type setScimAvailableRequest struct {
	Enabled bool `json:"enabled"`
}

// SetScimAvailable godoc
//
//	@Summary		Enable or disable SCIM for a workspace
//	@Description	When enabled is true and no ScimConfig exists, a blank one is created. When enabled is false, the ScimConfig (and the SCIM token) is removed.
//	@Tags			workspaces
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"Workspace ID"
//	@Param			body	body		setScimAvailableRequest	true	"enabled flag"
//	@Success		200		{object}	scimStateResponse
//	@Failure		400		{object}	internal.ErrorResponse	"invalid workspace id or request body"
//	@Failure		401		{object}	internal.ErrorResponse	"unauthorized"
//	@Failure		403		{object}	internal.ErrorResponse	"not approved or forbidden"
//	@Failure		404		{object}	internal.ErrorResponse	"workspace not found"
//	@Router			/workspaces/{id}/scim [patch]
func (h *Handler) SetScimAvailable(c echo.Context) error {
	wid, err := id.WorkspaceIDFrom(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid workspace id")
	}
	var req setScimAvailableRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	available, err := h.setScim.Execute(c.Request().Context(), wid, req.Enabled)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, scimStateResponse{ScimEnabled: available})
}