package presentation

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/admin/presentation/internal"
	"github.com/reearth/reearthx/log"
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
		log.Errorfc(ctx, "[admin] unhandled error: %v", err)
	}

	if jsonErr := c.JSON(status, internal.ErrorResponse{Code: code, Message: msg}); jsonErr != nil {
		log.Errorfc(ctx, "[admin] failed to write error response: %v", jsonErr)
	}
}
