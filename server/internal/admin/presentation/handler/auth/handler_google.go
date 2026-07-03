package auth

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

// GoogleSignIn godoc
//
//	@Summary		Google id_token でサインイン
//	@Description	Google の id_token を検証し、管理者セッション Cookie を発行する。新規アカウントは pending（bootstrap 対象なら approved）。
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		GoogleSignInRequest	true	"Google id_token"
//	@Success		200		{object}	GoogleSignInResponse
//	@Failure		400		{object}	internal.ErrorResponse	"リクエスト不正"
//	@Failure		401		{object}	internal.ErrorResponse	"id_token 検証失敗"
//	@Failure		403		{object}	internal.ErrorResponse	"ドメイン不許可 / 未検証メール"
//	@Router			/auth/google [post]
func (h *Handler) GoogleSignIn(c echo.Context) error {
	var req GoogleSignInRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := c.Validate(&req); err != nil {
		return err
	}

	ctx := c.Request().Context()
	u, err := h.signIn.Execute(ctx, req.IDToken)
	if err != nil {
		return err
	}

	now := time.Now()
	token, err := h.sess.Issue(u.ID(), now)
	if err != nil {
		return err
	}
	c.SetCookie(h.newSessionCookie(token, now))

	return c.JSON(http.StatusOK, newGoogleSignInResponse(u))
}
