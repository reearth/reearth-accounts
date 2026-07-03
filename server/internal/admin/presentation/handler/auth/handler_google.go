package auth

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

// GoogleSignIn godoc
//
//	@Summary		Sign in with a Google id_token
//	@Description	Verifies the Google id_token and issues an admin session cookie. New accounts are created as pending (approved when the email is bootstrapped).
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		GoogleSignInRequest	true	"Google id_token"
//	@Success		200		{object}	GoogleSignInResponse
//	@Failure		400		{object}	internal.ErrorResponse	"invalid request"
//	@Failure		401		{object}	internal.ErrorResponse	"id_token verification failed"
//	@Failure		403		{object}	internal.ErrorResponse	"domain not allowed / email not verified"
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
