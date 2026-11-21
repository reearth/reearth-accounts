package handlers

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/internal/adapter/http/httpmodel"
	httpinternal "github.com/reearth/reearth-accounts/server/internal/adapter/http/internal"
	"github.com/reearth/reearth-accounts/server/pkg/legal"
)

// AcceptMyPolicies godoc
// @Tags User
// @Summary Accept policy versions for the current user
// @Description Each version must be the current or upcoming version of its type. Accepting a version the user already holds, or a lower one, records nothing. Returns 404 when REEARTH_ACCOUNTS_POLICY_WEBHOOK_SECRET is unset.
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body httpmodel.AcceptPoliciesRequest true "versions to accept"
// @Success 200 {object} httpmodel.MyPoliciesResponse
// @Failure 400 {object} internal.ErrorResponse "unknown type, malformed version, or a version that is neither current nor upcoming"
// @Failure 401 {object} internal.ErrorResponse
// @Failure 404 {object} internal.ErrorResponse "policies not configured"
// @Router /api/users/me/policies/accept [post]
func (h *PolicyHandler) AcceptMyPolicies(c echo.Context) error {
	ctx := c.Request().Context()
	u, err := httpinternal.RequireUser(c)
	if err != nil {
		return err
	}
	req := &httpmodel.AcceptPoliciesRequest{}
	if err := httpinternal.BindValidate(c, req); err != nil {
		return err
	}
	uc := httpinternal.Usecases(c).Policy
	if _, err := uc.Accept(ctx, req.Inputs(), adapter.Client(ctx), httpinternal.Operator(c)); err != nil {
		return policyError(err)
	}
	ov, err := uc.Overview(ctx, httpinternal.Operator(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpmodel.NewMyPoliciesResponse(ov, u.Metadata().Lang().String()))
}

// GetMyPolicies godoc
// @Tags User
// @Summary Get the current user's policy acceptances, pending policies and upcoming policies
// @Description Every list is empty when REEARTH_ACCOUNTS_POLICY_WEBHOOK_SECRET is unset.
// @Security BearerAuth
// @Produce json
// @Success 200 {object} httpmodel.MyPoliciesResponse
// @Failure 401 {object} internal.ErrorResponse
// @Router /api/users/me/policies [get]
func (h *PolicyHandler) GetMyPolicies(c echo.Context) error {
	u, err := httpinternal.RequireUser(c)
	if err != nil {
		return err
	}
	ov, err := httpinternal.Usecases(c).Policy.Overview(c.Request().Context(), httpinternal.Operator(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpmodel.NewMyPoliciesResponse(ov, u.Metadata().Lang().String()))
}

func policyError(err error) error {
	switch {
	case errors.Is(err, legal.ErrNotConfigured):
		return httpinternal.NewError(http.StatusNotFound, err.Error(), err)
	case errors.Is(err, legal.ErrInvalidVersion), errors.Is(err, legal.ErrNotAcceptable), errors.Is(err, legal.ErrUnknownType):
		return httpinternal.NewError(http.StatusBadRequest, err.Error(), err)
	}
	return err
}
