package presentation

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/reearth-accounts-admin/presentation/internal"
	"github.com/reearth/reearth-accounts/server/internal/reearth-accounts-admin/usecase/useruc"
	"github.com/reearth/reearthx/log"
	"github.com/reearth/reearthx/rerror"
)

// CustomHTTPErrorHandler converts errors returned from handlers into the
// standard ErrorResponse JSON shape.
func CustomHTTPErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	ctx := c.Request().Context()
	status, code, msg := classify(err)

	if status >= http.StatusInternalServerError {
		log.Errorfc(ctx, "[admin] unhandled error: %v", err)
	}

	if jsonErr := c.JSON(status, internal.ErrorResponse{Code: code, Message: msg}); jsonErr != nil {
		log.Errorfc(ctx, "[admin] failed to write error response: %v", jsonErr)
	}
}

func classify(err error) (status int, code, msg string) {
	var echoErr *echo.HTTPError
	if errors.As(err, &echoErr) {
		m := http.StatusText(echoErr.Code)
		if s, ok := echoErr.Message.(string); ok {
			m = s
		}
		return echoErr.Code, http.StatusText(echoErr.Code), m
	}

	switch {
	case errors.Is(err, useruc.ErrOperationDenied):
		return http.StatusForbidden, http.StatusText(http.StatusForbidden), "operation denied"
	case errors.Is(err, rerror.ErrNotFound):
		return http.StatusNotFound, http.StatusText(http.StatusNotFound), "not found"
	case rerror.UnwrapErrInternal(err) != nil:
		return http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), "internal server error"
	default:
		return http.StatusBadRequest, http.StatusText(http.StatusBadRequest), err.Error()
	}
}
