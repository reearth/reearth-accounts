package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/adapter/http/httpmodel"
	httpinternal "github.com/reearth/reearth-accounts/server/internal/adapter/http/internal"
)

// IntegrationHandler serves the routes about the integration a client token
// acts for (ADR-0002 decision 5). The integration comes from the
// X-Reearth-Act-As header only, so a caller cannot name another integration.
type IntegrationHandler struct{}

func NewIntegrationHandler() *IntegrationHandler {
	return &IntegrationHandler{}
}

// Workspaces godoc
// @Tags Integration
// @Summary List the acting integration's workspaces and roles
// @Description Only a registered client token with X-Reearth-Act-As: integration:<id> may call this.
// @Security BearerAuth
// @Param X-Reearth-Act-As header string true "integration:<id>"
// @Produce json
// @Success 200 {object} httpmodel.IntegrationWorkspacesResponse
// @Failure 401 {object} internal.ErrorResponse
// @Failure 403 {object} internal.ErrorResponse
// @Router /api/integrations/self/workspaces [get]
func (h *IntegrationHandler) Workspaces(c echo.Context) error {
	op := httpinternal.Operator(c)
	ms, err := httpinternal.Usecases(c).Workspace.ActingIntegrationMemberships(c.Request().Context(), op)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpmodel.NewIntegrationWorkspacesResponse(*op.Integration, ms))
}

// LeaveAll godoc
// @Tags Integration
// @Summary Remove the acting integration from all its workspaces
// @Description Call this when the integration is deleted. Each workspace is left on its own, so the response is 200 even when some fail: they are listed in failed_workspace_ids. Retrying is safe and removes only what is left. Leaving needs no role. Only a registered client token with X-Reearth-Act-As: integration:<id> may call this.
// @Security BearerAuth
// @Param X-Reearth-Act-As header string true "integration:<id>"
// @Produce json
// @Success 200 {object} httpmodel.LeaveAllWorkspacesResponse
// @Failure 401 {object} internal.ErrorResponse
// @Failure 403 {object} internal.ErrorResponse
// @Router /api/integrations/self/workspaces [delete]
func (h *IntegrationHandler) LeaveAll(c echo.Context) error {
	res, err := httpinternal.Usecases(c).Workspace.LeaveAllAsIntegration(c.Request().Context(), httpinternal.Operator(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpmodel.NewLeaveAllWorkspacesResponse(res))
}
