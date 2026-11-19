package handlers

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/adapter/http/httpmodel"
	httpinternal "github.com/reearth/reearth-accounts/server/internal/adapter/http/internal"
	"github.com/reearth/reearth-accounts/server/pkg/legal"
)

type PolicyHandler struct{}

func NewPolicyHandler() *PolicyHandler { return &PolicyHandler{} }

// Webhook godoc
// @Tags Policies
// @Summary Receive policy versions from the policy repository
// @Description Takes the versions.json payload that a policy repository posts when it changes. The signature is the only check: X-Policy-Signature must be sha256=<hex HMAC-SHA256 of the raw body>. Returns 404 when REEARTH_ACCOUNTS_POLICY_WEBHOOK_SECRET is unset. A rejected payload saves nothing.
// @Accept json
// @Produce json
// @Param X-Policy-Signature header string true "sha256=<hex HMAC-SHA256 of the raw body>"
// @Success 200 {object} httpmodel.PolicyWebhookResponse
// @Failure 400 {object} internal.ErrorResponse "missing or malformed field, or timestamp more than 5 minutes ahead"
// @Failure 401 {object} internal.ErrorResponse "missing or wrong signature"
// @Failure 404 {object} internal.ErrorResponse "policies not configured"
// @Failure 413 {object} internal.ErrorResponse "body larger than 1 MiB"
// @Router /api/policies/webhook [post]
func (h *PolicyHandler) Webhook(c echo.Context) error {
	req := c.Request()
	res, err := httpinternal.Usecases(c).Policy.ReceiveWebhook(req.Context(), req.Body, req.Header.Get(legal.SignatureHeader))
	var pe *legal.PayloadError
	switch {
	case errors.Is(err, legal.ErrNotConfigured):
		return echo.ErrNotFound
	case errors.Is(err, legal.ErrPayloadTooLarge):
		return httpinternal.NewError(http.StatusRequestEntityTooLarge, "payload too large", err)
	case errors.Is(err, legal.ErrBadSignature):
		return httpinternal.NewError(http.StatusUnauthorized, "unauthorized", err)
	case errors.As(err, &pe):
		return httpinternal.NewError(http.StatusBadRequest, pe.Error(), err)
	case err != nil:
		return err
	}
	return c.JSON(http.StatusOK, httpmodel.NewPolicyWebhookResponse(res))
}
