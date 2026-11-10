package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/adapter/http/httpmodel"
	httpinternal "github.com/reearth/reearth-accounts/server/internal/adapter/http/internal"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
)

// memberUserExists looks up every user member of ws in one query.
func memberUserExists(c echo.Context, ws workspace.List) (func(user.ID) bool, error) {
	uniq := map[user.ID]struct{}{}
	for _, w := range ws {
		if w == nil {
			continue
		}
		for uid := range w.Members().Users() {
			uniq[uid] = struct{}{}
		}
	}
	existing := map[user.ID]struct{}{}
	has := func(id user.ID) bool {
		_, ok := existing[id]
		return ok
	}
	if len(uniq) == 0 {
		return has, nil
	}
	ids := make(user.IDList, 0, len(uniq))
	for uid := range uniq {
		ids = append(ids, uid)
	}
	ul, err := httpinternal.Usecases(c).User.FetchByID(c.Request().Context(), ids)
	if err != nil {
		return nil, err
	}
	for _, u := range ul {
		if u != nil {
			existing[u.ID()] = struct{}{}
		}
	}
	return has, nil
}

func (h *WorkspaceHandler) workspaceResponses(c echo.Context, ws workspace.List) ([]*httpmodel.WorkspaceResponse, error) {
	userExists, err := memberUserExists(c, ws)
	if err != nil {
		return nil, err
	}
	res := httpmodel.NewWorkspaceResponses(ws, userExists)
	signWorkspacePhotos(c.Request().Context(), h.storage, res)
	return res, nil
}

func (h *WorkspaceHandler) writeWorkspace(c echo.Context, w *workspace.Workspace) error {
	res, err := h.workspaceResponses(c, workspace.List{w})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, res[0])
}

func (h *WorkspaceHandler) writeWorkspaces(c echo.Context, ws workspace.List) error {
	res, err := h.workspaceResponses(c, ws)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, res)
}
