package workspace

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/admin/presentation/internal"
	"github.com/reearth/reearth-accounts/server/pkg/pagination"
)

// ListWorkspaces godoc
//
//	@Summary		List workspaces
//	@Description	Lists workspaces across all tenants, optionally filtered by a name/alias keyword, with offset pagination.
//	@Tags			workspaces
//	@Produce		json
//	@Param			q			query		string	false	"Search by name or alias"
//	@Param			page		query		int		false	"Page number (1-based)"
//	@Param			per_page	query		int		false	"Items per page (max 100)"
//	@Success		200			{object}	ListWorkspacesResponse
//	@Failure		400			{object}	internal.ErrorResponse	"invalid query"
//	@Failure		401			{object}	internal.ErrorResponse	"unauthorized"
//	@Failure		403			{object}	internal.ErrorResponse	"not approved"
//	@Router			/workspaces [get]
func (h *Handler) ListWorkspaces(c echo.Context) error {
	var keyword *string
	if q := c.QueryParam("q"); q != "" {
		keyword = &q
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

	list, pi, err := h.list.Execute(c.Request().Context(), keyword, p)
	if err != nil {
		return err
	}

	effectivePage := int64(1)
	if page > 0 {
		effectivePage = page
	}
	return c.JSON(http.StatusOK, ListWorkspacesResponse{
		Items:      newWorkspaceResponses(list),
		TotalCount: pi.TotalCount,
		Page:       effectivePage,
		PerPage:    p.Offset.Limit,
	})
}
