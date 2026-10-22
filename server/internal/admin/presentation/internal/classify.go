package internal

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/adminuseruc"
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/authuc"
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/workspaceuc"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/rerror"
	"github.com/reearth/reearthx/usecasex"
)

// Classify maps an error returned by a handler to its HTTP status and the
// code and message of the ErrorResponse body.
func Classify(err error) (status int, code, msg string) {
	var echoErr *echo.HTTPError
	if errors.As(err, &echoErr) {
		m := http.StatusText(echoErr.Code)
		if s, ok := echoErr.Message.(string); ok {
			m = s
		}
		return echoErr.Code, http.StatusText(echoErr.Code), m
	}

	switch {
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
	case errors.Is(err, adminuseruc.ErrLastSystemAdmin):
		return http.StatusBadRequest, http.StatusText(http.StatusBadRequest), "cannot demote the last system admin"
	case errors.Is(err, user.ErrCursorPaginationUnsupported):
		return http.StatusBadRequest, http.StatusText(http.StatusBadRequest), "cursor pagination is not supported"
	case errors.Is(err, workspace.ErrCursorPaginationUnsupported):
		return http.StatusBadRequest, http.StatusText(http.StatusBadRequest), "cursor pagination is not supported"
	case errors.Is(err, workspace.ErrNotImplemented):
		return http.StatusNotImplemented, http.StatusText(http.StatusNotImplemented), "not implemented on this backend"
	case errors.Is(err, workspaceuc.ErrAlreadyOwner):
		return http.StatusConflict, "already_owner", "target user is already the sole owner"
	case errors.Is(err, workspaceuc.ErrPersonalWorkspace):
		return http.StatusUnprocessableEntity, "personal_workspace", "personal workspace ownership cannot be transferred"
	case errors.Is(err, workspaceuc.ErrTargetNotMember):
		return http.StatusUnprocessableEntity, "target_not_member", "target user is not a member of the workspace"
	case errors.Is(err, workspaceuc.ErrTargetDisabled):
		return http.StatusUnprocessableEntity, "target_disabled", "target member is disabled"
	case errors.Is(err, workspaceuc.ErrTargetUserNotFound):
		return http.StatusUnprocessableEntity, "target_user_not_found", "target user not found"
	case errors.Is(err, usecasex.ErrTransaction):
		return http.StatusConflict, "conflict", "the resource was changed concurrently; reload and retry"
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
