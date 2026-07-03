package auth

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// Logout godoc
//
//	@Summary		ログアウト
//	@Description	セッション Cookie を破棄する。公開エンドポイント（期限切れ/無効なトークンでも Cookie を消去できるようにするため）。
//	@Tags			auth
//	@Produce		json
//	@Success		204	"No Content"
//	@Router			/auth/logout [post]
func (h *Handler) Logout(c echo.Context) error {
	c.SetCookie(h.clearSessionCookie())
	return c.NoContent(http.StatusNoContent)
}
