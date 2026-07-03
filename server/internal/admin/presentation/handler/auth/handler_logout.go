package auth

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// Logout godoc
//
//	@Summary		Log out
//	@Description	Clears the session cookie. Public endpoint so the cookie can be cleared even with an expired/invalid token.
//	@Tags			auth
//	@Produce		json
//	@Success		204	"No Content"
//	@Router			/auth/logout [post]
func (h *Handler) Logout(c echo.Context) error {
	c.SetCookie(h.clearSessionCookie())
	return c.NoContent(http.StatusNoContent)
}
