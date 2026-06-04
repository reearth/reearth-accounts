package user

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/reearth-accounts-admin/presentation/internal"
	_ "github.com/reearth/reearth-accounts/server/internal/reearth-accounts-admin/usecase/useruc" // for swagger
)

// ListUsers godoc
//
//	@Summary		ユーザー一覧を取得
//	@Description	全ユーザーを取得する（管理者権限が必要）
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	useruc.ListUsersOutput
//	@Failure		401	{object}	internal.ErrorResponse	"認証エラー"
//	@Failure		403	{object}	internal.ErrorResponse	"権限エラー"
//	@Failure		500	{object}	internal.ErrorResponse	"サーバーエラー"
//	@Security		BearerAuth
//	@Router			/users [get]
func (h *Handler) ListUsers(c echo.Context) error {
	operator, err := internal.GetUser(c)
	if err != nil {
		return err
	}

	output, err := h.listUC.Execute(c.Request().Context(), operator)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, output)
}
