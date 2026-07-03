package auth

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/admin/presentation/internal"
)

// Me godoc
//
// Authenticates via the admin_session HttpOnly cookie. Swagger 2.0 (what swag
// emits) has no cookie-auth security type, so there is no @Security annotation
// here; the browser sends the cookie automatically.
//
//	@Summary		Get the current admin user
//	@Description	Returns the admin user record for the session cookie (any status).
//	@Tags			auth
//	@Produce		json
//	@Success		200	{object}	MeResponse
//	@Failure		401	{object}	internal.ErrorResponse	"unauthorized"
//	@Failure		404	{object}	internal.ErrorResponse	"account not found"
//	@Router			/me [get]
func (h *Handler) Me(c echo.Context) error {
	id, err := internal.GetSessionAdminUserID(c)
	if err != nil {
		return err
	}

	u, err := h.getMe.Execute(c.Request().Context(), id)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, newMeResponse(u))
}
