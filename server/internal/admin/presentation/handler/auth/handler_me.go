package auth

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/admin/presentation/internal"
)

// Me godoc
//
//	@Summary		現在の管理者ユーザーを取得
//	@Description	セッション Cookie に対応する管理者ユーザーのレコードを返す（status を問わない）。
//	@Tags			auth
//	@Produce		json
//	@Success		200	{object}	MeResponse
//	@Failure		401	{object}	internal.ErrorResponse	"未認証"
//	@Failure		404	{object}	internal.ErrorResponse	"アカウントが存在しない"
//	@Router			/me [get]

// Note: this endpoint authenticates via the admin_session HttpOnly cookie.
// Swagger 2.0 (what swag emits) has no cookie-auth security type, so there is
// no @Security annotation here; the cookie is sent automatically by the browser.
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
