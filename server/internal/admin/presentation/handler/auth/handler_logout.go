package auth

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// Logout godoc
//
//	@Summary		Log out
//	@Description	Clears the admin session cookie and the admin_csrf double-submit cookie. Public endpoint so the cookies can be cleared even with an expired/invalid token.
//	@Tags			auth
//	@Produce		json
//	@Success		204	"No Content"
//	@Router			/auth/logout [post]
func (h *Handler) Logout(c echo.Context) error {
	c.SetCookie(h.clearSessionCookie())
	c.SetCookie(h.clearCSRFCookie())
	return c.NoContent(http.StatusNoContent)
}
