package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/adapter/http/httpmodel"
	httpinternal "github.com/reearth/reearth-accounts/server/internal/adapter/http/internal"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
)

// memberUsers looks up every user member of ws in one query.
func memberUsers(c echo.Context, ws workspace.List) (map[user.ID]*user.User, error) {
	uniq := map[user.ID]struct{}{}
	for _, w := range ws {
		if w == nil {
			continue
		}
		for uid := range w.Members().Users() {
			uniq[uid] = struct{}{}
		}
	}
	found := map[user.ID]*user.User{}
	if len(uniq) == 0 {
		return found, nil
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
			found[u.ID()] = u
		}
	}
	return found, nil
}

func workspaceResponses(c echo.Context, storage gateway.Storage, ws workspace.List) ([]*httpmodel.WorkspaceResponse, error) {
	users, err := memberUsers(c, ws)
	if err != nil {
		return nil, err
	}
	var embedFor *httpmodel.EmailViewer
	if requestedExpansions(c).has(ExpandMemberUsers) {
		v := httpmodel.ViewerOf(httpinternal.User(c))
		embedFor = &v
	}
	res := httpmodel.NewWorkspaceResponses(ws, users, embedFor)
	signWorkspacePhotos(c.Request().Context(), storage, res)
	return res, nil
}

func (h *WorkspaceHandler) writeWorkspace(c echo.Context, w *workspace.Workspace) error {
	res, err := workspaceResponses(c, h.storage, workspace.List{w})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, res[0])
}

func (h *WorkspaceHandler) writeWorkspaces(c echo.Context, ws workspace.List) error {
	res, err := workspaceResponses(c, h.storage, ws)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, res)
}
