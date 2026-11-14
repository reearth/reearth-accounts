package presentation

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/admin/presentation/internal"
	"github.com/reearth/reearth-accounts/server/pkg/applog"
)

// CustomHTTPErrorHandler converts errors returned from handlers into the
// standard ErrorResponse JSON shape.
func CustomHTTPErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	ctx := c.Request().Context()
	status, code, msg := internal.Classify(err)

	if status >= http.StatusInternalServerError {
		applog.Error(ctx, "[admin] unhandled error", applog.Err(err))
	}

	if jsonErr := c.JSON(status, internal.ErrorResponse{Code: code, Message: msg}); jsonErr != nil {
		applog.Error(ctx, "[admin] failed to write error response", applog.Err(jsonErr))
	}
}
