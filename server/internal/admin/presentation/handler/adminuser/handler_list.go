package adminuser

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
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

	page := parseInt(c.QueryParam("page"))
	perPage := parseInt(c.QueryParam("per_page"))
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

// parseInt parses a non-negative int64, returning 0 for empty or invalid input.
// Callers treat 0 as the "use the default" sentinel (pagination.ToPagination
// applies its own page/per_page defaults).
func parseInt(s string) int64 {
	if s == "" {
		return 0
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 {
		return 0
	}
	return n
}
