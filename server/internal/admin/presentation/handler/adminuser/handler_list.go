package adminuser

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/admin/presentation/internal"
	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
	"github.com/reearth/reearth-accounts/server/pkg/pagination"
)

// ListAdminUsers godoc
//
//	@Summary		List admin users
//	@Description	Lists admin users in creation order, optionally filtered by status, with offset pagination.
//	@Tags			admin-users
//	@Produce		json
//	@Param			status		query		string	false	"Filter by status"	Enums(pending, approved, rejected)
//	@Param			page		query		int		false	"Page number (1-based)"
//	@Param			per_page	query		int		false	"Items per page (max 100)"
//	@Success		200			{object}	ListAdminUsersResponse
//	@Failure		400			{object}	internal.ErrorResponse	"invalid query"
//	@Failure		401			{object}	internal.ErrorResponse	"unauthorized"
//	@Failure		403			{object}	internal.ErrorResponse	"not approved"
//	@Router			/admin-users [get]
func (h *Handler) ListAdminUsers(c echo.Context) error {
	var filter adminuser.ListFilter

	if s := c.QueryParam("status"); s != "" {
		status, err := adminuser.StatusFrom(s)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid status")
		}
		filter.Status = &status
	}

	page, err := internal.ParsePageParam(c.QueryParam("page"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid page")
	}
	perPage, err := internal.ParsePageParam(c.QueryParam("per_page"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid per_page")
	}
	p := pagination.ToPagination(page, perPage)
	filter.Pagination = p

	list, pi, err := h.list.Execute(c.Request().Context(), filter)
	if err != nil {
		return err
	}

	effectivePage := int64(1)
	if page > 0 {
		effectivePage = page
	}
	return c.JSON(http.StatusOK, ListAdminUsersResponse{
		Items:      newAdminUserResponses(list),
		TotalCount: pi.TotalCount,
		Page:       effectivePage,
		PerPage:    p.Offset.Limit,
	})
}
