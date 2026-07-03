package auth

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// Logout godoc
//
//	@Summary		ログアウト
//	@Description	セッション Cookie を破棄する。
//	@Tags			auth
//	@Produce		json
//	@Success		204	"No Content"
//	@Failure		401	{object}	internal.ErrorResponse	"未認証"
//	@Security		AdminSession
//	@Router			/auth/logout [post]
func (h *Handler) Logout(c echo.Context) error {
	c.SetCookie(h.clearSessionCookie())
	return c.NoContent(http.StatusNoContent)
}
