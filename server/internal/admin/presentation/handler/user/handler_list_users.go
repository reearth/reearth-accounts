package user

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/admin/presentation/internal"
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/useruc"
)

// ListUsers godoc
//
//	@Summary		ユーザー一覧を取得
//	@Description	ユーザーをページネーション付きで取得する（管理者権限が必要）
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			page		query		int	false	"ページ番号（デフォルト: 1）"
//	@Param			page_size	query		int	false	"1ページあたりの件数（デフォルト: 50、最大: 100）"
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

	var params internal.PageParams
	if err := c.Bind(&params); err != nil {
		return err
	}
	page, pageSize := params.Normalized()

	input := useruc.ListUsersInput{
		Page:     int64(page),
		PageSize: int64(pageSize),
	}

	output, err := h.listUC.Execute(c.Request().Context(), operator, input)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, output)
}
