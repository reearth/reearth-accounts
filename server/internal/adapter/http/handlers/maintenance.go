package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/adapter/http/httpmodel"
	httpinternal "github.com/reearth/reearth-accounts/server/internal/adapter/http/internal"
	"github.com/reearth/reearth-accounts/server/internal/usecase/maintenance"
)

// maintenanceBudget matches the default interval of the per-instance loop.
const maintenanceBudget = 30 * time.Second

type PassRunner interface {
	RunPass(ctx context.Context) (maintenance.Report, error)
}

type MaintenanceHandler struct {
	key    maintenance.Key
	runner PassRunner
}

func NewMaintenanceHandler(key maintenance.Key, runner PassRunner) *MaintenanceHandler {
	return &MaintenanceHandler{key: key, runner: runner}
}

// Run godoc
// @Tags Maintenance
// @Summary Run one maintenance pass
// @Description Sweeps the mail outbox, runs retention steps and sends due mail within 30 seconds, then reports the counts. For a scheduler. Returns 404 when REEARTH_ACCOUNTS_MAINTENANCE_KEY is unset.
// @Produce json
// @Param X-Accounts-Maintenance-Key header string true "the value of REEARTH_ACCOUNTS_MAINTENANCE_KEY"
// @Success 200 {object} httpmodel.MaintenanceRunResponse
// @Failure 401 {object} internal.ErrorResponse "missing or wrong key"
// @Failure 404 {object} internal.ErrorResponse "maintenance route not configured"
// @Router /api/maintenance/run [post]
func (h *MaintenanceHandler) Run(c echo.Context) error {
	if !h.key.Enabled() || h.runner == nil {
		return echo.ErrNotFound
	}
	if !h.key.Matches(c.Request().Header.Get(maintenance.KeyHeader)) {
		return httpinternal.NewError(http.StatusUnauthorized, "unauthorized", nil)
	}
	ctx, cancel := context.WithTimeout(c.Request().Context(), maintenanceBudget)
	defer cancel()
	rep, err := h.runner.RunPass(ctx)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpmodel.NewMaintenanceRunResponse(rep))
}
