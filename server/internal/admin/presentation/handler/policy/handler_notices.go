package policy

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/admin/presentation/internal"
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/policyuc"
	"github.com/reearth/reearth-accounts/server/pkg/applog"
	"github.com/reearth/reearth-accounts/server/pkg/legal"
)

type CreatePolicyNoticeRequest struct {
	Audience string `json:"audience" validate:"required" enums:"all,owners" example:"all"`
} // @name CreatePolicyNoticeRequest

// CreatePolicyNotice godoc
//
//	@Summary		Record a notice for a policy's upcoming version
//	@Description	Records a notice for the upcoming version of the policy type. The accounts server then queues one mail per user of the audience and sends it: "all" is every active user with an email address, "owners" is the owners of non-personal workspaces. A major notice skips users who already accepted the version. Each user gets at most one mail per version, so repeating the request queues nothing new and returns the recorded notice with 200. The admin API sends no mail itself.
//	@Tags			policies
//	@Accept			json
//	@Produce		json
//	@Param			type	path		string						true	"Policy type"	example(tos)
//	@Param			body	body		CreatePolicyNoticeRequest	true	"Audience"
//	@Success		200		{object}	CreatePolicyNoticeResponse	"already recorded"
//	@Success		201		{object}	CreatePolicyNoticeResponse	"recorded"
//	@Failure		400		{object}	internal.ErrorResponse		"invalid type / invalid request body / invalid audience"
//	@Failure		401		{object}	internal.ErrorResponse		"unauthorized"
//	@Failure		403		{object}	internal.ErrorResponse		"not approved / forbidden"
//	@Failure		422		{object}	internal.ErrorResponse		"no_upcoming_version / not_required / patch_version"
//	@Router			/policies/{type}/notices [post]
func (h *Handler) CreatePolicyNotice(c echo.Context) error {
	admin, err := internal.GetAdminUser(c)
	if err != nil {
		return err
	}
	t, err := legal.ParseType(c.Param("type"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid type")
	}
	var body CreatePolicyNoticeRequest
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := c.Validate(&body); err != nil {
		return err
	}
	a, err := legal.ParseAudience(body.Audience)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid audience")
	}

	ctx := c.Request().Context()
	n, created, err := h.create.Execute(ctx, policyuc.CreateNoticeInput{Audience: a, RequestedBy: admin.ID(), Type: t})
	if err != nil {
		status, code, _ := internal.Classify(err)
		applog.Info(ctx, "[admin] create policy notice rejected", applog.AdminID(admin.ID()), applog.PolicyType(string(t)), applog.Status(status), applog.Code(code))
		return err
	}
	applog.Info(ctx, "[admin] create policy notice", applog.AdminID(admin.ID()), applog.NoticeID(n.ID), applog.PolicyType(string(t)), applog.Created(created))

	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	return c.JSON(status, CreatePolicyNoticeResponse{Created: created, Notice: newPolicyNoticeResponse(n)})
}

// ListPolicyNotices godoc
//
//	@Summary		List the notices of a policy's upcoming version
//	@Description	Returns each notice recorded for the upcoming version of the policy type, with the number of its mail records in each outbox state. The list is empty when the type has no upcoming version. While the accounts server has no mail provider, records stay pending.
//	@Tags			policies
//	@Produce		json
//	@Param			type	path		string	true	"Policy type"	example(tos)
//	@Success		200		{object}	ListPolicyNoticesResponse
//	@Failure		400		{object}	internal.ErrorResponse	"invalid type"
//	@Failure		401		{object}	internal.ErrorResponse	"unauthorized"
//	@Failure		403		{object}	internal.ErrorResponse	"not approved / forbidden"
//	@Router			/policies/{type}/notices [get]
func (h *Handler) ListPolicyNotices(c echo.Context) error {
	t, err := legal.ParseType(c.Param("type"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid type")
	}
	ss, err := h.list.Execute(c.Request().Context(), t)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, newListPolicyNoticesResponse(ss))
}
