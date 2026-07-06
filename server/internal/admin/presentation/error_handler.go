package presentation

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/admin/presentation/internal"
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/adminuseruc"
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/authuc"
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/useruc"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
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
	case errors.Is(err, authuc.ErrInvalidToken):
		return http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized), "invalid id token"
	case errors.Is(err, authuc.ErrEmailNotVerified):
		return http.StatusForbidden, http.StatusText(http.StatusForbidden), "email not verified"
	case errors.Is(err, authuc.ErrDomainNotAllowed):
		return http.StatusForbidden, http.StatusText(http.StatusForbidden), "email domain not allowed"
	case errors.Is(err, adminuseruc.ErrCannotModifySelf):
		return http.StatusBadRequest, http.StatusText(http.StatusBadRequest), "cannot modify your own admin account"
	case errors.Is(err, adminuseruc.ErrLastApprovedAdmin):
		return http.StatusBadRequest, http.StatusText(http.StatusBadRequest), "cannot reject the last approved admin"
	case errors.Is(err, workspace.ErrNotImplemented):
		return http.StatusNotImplemented, http.StatusText(http.StatusNotImplemented), "not implemented on this backend"
	case errors.Is(err, rerror.ErrNotFound):
		return http.StatusNotFound, http.StatusText(http.StatusNotFound), "not found"
	default:
		// Unrecognized errors (DB/network failures, programming errors, ...) are
		// treated as server-side faults: respond with a generic 500 so internal
		// details are never leaked to clients. The original error is logged by
		// CustomHTTPErrorHandler. Handlers that mean "bad request" should return
		// an *echo.HTTPError with the appropriate 4xx code.
		return http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), "internal server error"
	}
}
