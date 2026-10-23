package auth

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// Logout godoc
//
//	@Summary		Log out
//	@Description	Clears the admin session cookie and the admin_csrf double-submit cookie. Public endpoint so the cookies can be cleared even with an expired/invalid token. When an admin_csrf cookie is sent, the X-Csrf-Token header must match it. This only rejects forged same-site requests: a cross-site request arrives without the SameSite=Lax admin_csrf cookie and is not CSRF-checked.
//	@Tags			auth
//	@Produce		json
//	@Param			X-Csrf-Token	header	string	false	"admin_csrf cookie value; required when that cookie is sent"
//	@Success		204	"No Content"
//	@Failure		403	{object}	internal.ErrorResponse	"missing or invalid csrf token"
//	@Router			/auth/logout [post]
func (h *Handler) Logout(c echo.Context) error {
	c.SetCookie(h.clearSessionCookie())
	c.SetCookie(h.clearCSRFCookie())
	return c.NoContent(http.StatusNoContent)
}
