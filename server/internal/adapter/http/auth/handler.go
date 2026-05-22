package auth

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/internal/adapter/http/httpmodel"
	httpinternal "github.com/reearth/reearth-accounts/server/internal/adapter/http/internal"
)

type Handler struct {
	authConfig adapter.Auth0ConfigProvider
}

func NewHandler(authConfig adapter.Auth0ConfigProvider) *Handler {
	return &Handler{authConfig: authConfig}
}

// Config godoc
// @Tags Auth
// @Summary Get public auth configuration
// @Produce json
// @Success 200 {object} httpmodel.AuthConfigResponse
// @Router /api/auth/config [get]
func (h *Handler) Config(c echo.Context) error {
	data := adapter.ExtractAuthConfigData(h.authConfig)
	return c.JSON(http.StatusOK, httpmodel.NewAuthConfigResponse(data))
}

// Logout godoc
// @Tags Auth
// @Summary Log out the current user
// @Security BearerAuth
// @Produce json
// @Success 200 {object} httpmodel.MeResponse
// @Failure 401 {object} internal.ErrorResponse
// @Router /api/auth/logout [post]
func (h *Handler) Logout(c echo.Context) error {
	ctx := c.Request().Context()
	op := httpinternal.Operator(c)
	u, err := httpinternal.Usecases(c).User.Logout(ctx, op)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpmodel.NewMeResponse(u))
}
