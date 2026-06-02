package internal

import (
	"errors"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearthx/log"
	"github.com/reearth/reearthx/rerror"
)

// ErrUnauthorized is returned by auth middleware when no/invalid credentials are present.
var ErrUnauthorized = errors.New("unauthorized")

// ErrForbidden is returned when the operator lacks permission.
var ErrForbidden = errors.New("forbidden")

// ErrorResponse is the structured error body returned for all REST failures.
type ErrorResponse struct {
	Status      int                  `json:"status"`
	Message     string               `json:"message"`
	Description string               `json:"description"`
	Err         error                `json:"-"`
	FieldErrors []FieldErrorResponse `json:"field_errors,omitempty"`
}

// FieldErrorResponse describes a single validation failure.
type FieldErrorResponse struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *ErrorResponse) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Message
}

// NewError builds an ErrorResponse with the given status/message.
func NewError(status int, message string, err error) *ErrorResponse {
	return &ErrorResponse{Status: status, Message: message, Description: message, Err: err}
}

// CustomHTTPErrorHandler maps any error returned from a handler to a structured JSON ErrorResponse.
func CustomHTTPErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	resp := mapError(err)

	if resp.Err != nil {
		// 5xx surfaces a genuine server-side failure worth flagging at error level;
		// 4xx client errors (validation, conflict, forbidden, ...) log at warn so
		// they don't drown the real errors.
		ctx := c.Request().Context()
		if resp.Status >= http.StatusInternalServerError {
			log.Errorfc(ctx, "rest: %d %s: %v", resp.Status, resp.Message, resp.Err)
		} else {
			log.Warnfc(ctx, "rest: %d %s: %v", resp.Status, resp.Message, resp.Err)
		}
	}
	if jsonErr := c.JSON(resp.Status, resp); jsonErr != nil {
		log.Errorfc(c.Request().Context(), "rest: failed to write error response: %v", jsonErr)
	}
}

func mapError(err error) *ErrorResponse {
	// Already structured.
	var er *ErrorResponse
	if errors.As(err, &er) {
		return er
	}

	// Echo's own errors (binding, 404 route, etc.).
	var he *echo.HTTPError
	if errors.As(err, &he) {
		msg := http.StatusText(he.Code)
		if m, ok := he.Message.(string); ok && m != "" {
			msg = m
		}
		return &ErrorResponse{Status: he.Code, Message: msg, Description: msg, Err: he.Internal}
	}

	// Validation errors.
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		fes := make([]FieldErrorResponse, 0, len(ve))
		for _, fe := range ve {
			fes = append(fes, FieldErrorResponse{Field: fe.Field(), Message: translateFieldError(fe)})
		}
		return &ErrorResponse{Status: http.StatusBadRequest, Message: "validation failed", Description: "one or more fields are invalid", FieldErrors: fes, Err: err}
	}

	switch {
	case errors.Is(err, rerror.ErrNotFound):
		return &ErrorResponse{Status: http.StatusNotFound, Message: "not found", Description: "the requested resource was not found"}
	case errors.Is(err, interfaces.ErrUserAlreadyExists),
		errors.Is(err, interfaces.ErrUserAliasAlreadyExists),
		errors.Is(err, interfaces.ErrWorkspaceAliasAlreadyExists):
		return &ErrorResponse{Status: http.StatusConflict, Message: "conflict", Description: err.Error(), Err: err}
	case errors.Is(err, ErrForbidden),
		errors.Is(err, interfaces.ErrPermissionDenied),
		errors.Is(err, interfaces.ErrOperationDenied),
		errors.Is(err, interfaces.ErrCannotChangeOwnerRole),
		errors.Is(err, interfaces.ErrCannotSelfPromote),
		errors.Is(err, interfaces.ErrOwnerCannotLeaveTheWorkspace):
		return &ErrorResponse{Status: http.StatusForbidden, Message: "forbidden", Description: err.Error(), Err: err}
	case errors.Is(err, ErrUnauthorized),
		errors.Is(err, interfaces.ErrInvalidOperator):
		return &ErrorResponse{Status: http.StatusUnauthorized, Message: "unauthorized", Description: "authentication is required"}
	case errors.Is(err, interfaces.ErrInvalidUserEmail),
		errors.Is(err, interfaces.ErrInvalidEmailOrPassword),
		errors.Is(err, interfaces.ErrUserInvalidPasswordConfirmation),
		errors.Is(err, interfaces.ErrUserInvalidPasswordReset),
		errors.Is(err, interfaces.ErrUserInvalidLang),
		errors.Is(err, interfaces.ErrSignupInvalidSecret),
		errors.Is(err, interfaces.ErrInvalidPhotoURL),
		errors.Is(err, interfaces.ErrNotVerifiedUser):
		return &ErrorResponse{Status: http.StatusBadRequest, Message: "bad request", Description: err.Error(), Err: err}
	default:
		return &ErrorResponse{Status: http.StatusInternalServerError, Message: "internal server error", Description: "an unexpected error occurred", Err: err}
	}
}
