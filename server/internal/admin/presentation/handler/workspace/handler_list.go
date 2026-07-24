package workspace

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/admin/presentation/internal"
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/workspaceuc"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/pagination"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
)

// maxWorkspaceIDsPerRequest caps a single batch-by-IDs request.
const maxWorkspaceIDsPerRequest = 100

// ListWorkspaces godoc
//
//	@Summary		List workspaces
//	@Description	Lists workspaces across all tenants, optionally filtered by a name/alias keyword, with offset pagination.
//	@Description	When one or more `ids` query parameters are supplied, the endpoint instead resolves those workspaces by ID (existing ones only; unknown IDs are omitted) and ignores `q`, `page` and `per_page`. At most 100 ids may be supplied per request.
//	@Tags			workspaces
//	@Produce		json
//	@Param			ids			query		[]string	false	"Batch fetch by workspace ID (repeatable, max 100). When present, q/page/per_page are ignored."	collectionFormat(multi)
//	@Param			q			query		string		false	"Search by name or alias"
//	@Param			page		query		int			false	"Page number (1-based)"
//	@Param			per_page	query		int			false	"Items per page (max 100)"
//	@Success		200			{object}	ListWorkspacesResponse
//	@Failure		400			{object}	internal.ErrorResponse	"invalid query"
//	@Failure		401			{object}	internal.ErrorResponse	"unauthorized"
//	@Failure		403			{object}	internal.ErrorResponse	"not approved"
//	@Failure		501			{object}	internal.ErrorResponse	"not implemented on this backend"
//	@Router			/workspaces [get]
func (h *Handler) ListWorkspaces(c echo.Context) error {
	if rawIDs := nonEmpty(c.QueryParams()["ids"]); len(rawIDs) > 0 {
		return h.listWorkspacesByIDs(c, rawIDs)
	}

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

	list, pi, err := h.list.Execute(c.Request().Context(), workspaceuc.ListWorkspacesInput{
		Keyword:    keyword,
		Pagination: p,
	})
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

// listWorkspacesByIDs resolves the given workspace IDs in one call. Malformed
// IDs are rejected with 400; unknown-but-valid IDs are omitted.
func (h *Handler) listWorkspacesByIDs(c echo.Context, rawIDs []string) error {
	if len(rawIDs) > maxWorkspaceIDsPerRequest {
		return echo.NewHTTPError(http.StatusBadRequest, "too many ids")
	}

	ids := make(workspace.IDList, 0, len(rawIDs))
	for _, raw := range rawIDs {
		wid, err := id.WorkspaceIDFrom(raw)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
		}
		ids = append(ids, wid)
	}

	list, _, err := h.list.Execute(c.Request().Context(), workspaceuc.ListWorkspacesInput{IDs: ids})
	if err != nil {
		return err
	}

	items := newWorkspaceResponses(list)
	return c.JSON(http.StatusOK, ListWorkspacesResponse{
		Items:      items,
		TotalCount: int64(len(items)),
		Page:       1,
		PerPage:    int64(len(items)),
	})
}

// nonEmpty drops empty strings, so a blank `?ids=` falls through to the normal
// keyword/pagination listing.
func nonEmpty(values []string) []string {
	out := values[:0:0]
	for _, v := range values {
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}
