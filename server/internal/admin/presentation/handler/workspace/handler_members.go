package workspace

import (
	"net/http"
	"sort"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/pkg/id"
)

// GetWorkspaceMembers godoc
//
//	@Summary		List a workspace's members
//	@Description	Returns the members of a workspace, each with their role and (when resolvable) the underlying user's name and email.
//	@Tags			workspaces
//	@Produce		json
//	@Param			id	path		string	true	"Workspace ID"
//	@Success		200	{array}		WorkspaceMemberResponse
//	@Failure		400	{object}	internal.ErrorResponse	"invalid id"
//	@Failure		401	{object}	internal.ErrorResponse	"unauthorized"
//	@Failure		403	{object}	internal.ErrorResponse	"not approved"
//	@Failure		404	{object}	internal.ErrorResponse	"not found"
//	@Router			/workspaces/{id}/members [get]
func (h *Handler) GetWorkspaceMembers(c echo.Context) error {
	wid, err := id.WorkspaceIDFrom(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}

	ws, users, err := h.members.Execute(c.Request().Context(), wid)
	if err != nil {
		return err
	}

	res := make([]WorkspaceMemberResponse, 0)
	if m := ws.Members(); m != nil {
		// Iterate ids + per-id lookup instead of Users(), which clones the map.
		for _, uid := range m.UserIDs() {
			member := m.User(uid)
			if member == nil {
				continue
			}
			item := WorkspaceMemberResponse{
				UserID:   uid.String(),
				Role:     string(member.Role),
				Disabled: member.Disabled,
			}
			if u, ok := users[uid]; ok {
				item.Name = u.Name()
				item.Email = u.Email()
			}
			res = append(res, item)
		}
	}
	sort.Slice(res, func(i, j int) bool { return res[i].UserID < res[j].UserID })

	return c.JSON(http.StatusOK, res)
}
