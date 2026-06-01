package handlers

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/adapter/http/httpmodel"
	httpinternal "github.com/reearth/reearth-accounts/server/internal/adapter/http/internal"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/pkg/id"
)

type WorkspaceHandler struct{}

func NewWorkspaceHandler() *WorkspaceHandler { return &WorkspaceHandler{} }

func badRequest(msg string) error {
	return httpinternal.NewError(http.StatusBadRequest, msg, nil)
}

// Create godoc
// @Tags Workspace
// @Summary Create a workspace
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body httpmodel.CreateWorkspaceRequest true "workspace fields"
// @Success 200 {object} httpmodel.WorkspaceResponse
// @Router /api/workspaces [post]
func (h *WorkspaceHandler) Create(c echo.Context) error {
	ctx := c.Request().Context()
	req := &httpmodel.CreateWorkspaceRequest{}
	if err := httpinternal.BindValidate(c, req); err != nil {
		return err
	}
	u, err := httpinternal.RequireUser(c)
	if err != nil {
		return err
	}
	desc := ""
	if req.Description != nil {
		desc = *req.Description
	}
	w, err := httpinternal.Usecases(c).Workspace.Create(ctx, req.Alias, req.Name, desc, u.ID(), httpinternal.Operator(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpmodel.NewWorkspaceResponse(w))
}

// Get godoc
// @Tags Workspace
// @Summary Get a workspace by ID
// @Security BearerAuth
// @Param id path string true "workspace ID"
// @Produce json
// @Success 200 {object} httpmodel.WorkspaceResponse
// @Failure 404 {object} internal.ErrorResponse
// @Router /api/workspaces/{id} [get]
func (h *WorkspaceHandler) Get(c echo.Context) error {
	ctx := c.Request().Context()
	wid, err := id.WorkspaceIDFrom(c.Param("id"))
	if err != nil {
		return badRequest("invalid workspace id")
	}
	w, err := httpinternal.Usecases(c).Workspace.FetchByID(ctx, wid)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpmodel.NewWorkspaceResponse(w))
}

// List godoc
// @Tags Workspace
// @Summary List workspaces by a single selector (ids|name|alias|user_id)
// @Description Response shape depends on the selector: ids and user_id return an array of workspaces; name and alias return a single workspace object; user_id with page/page_size returns a paginated object {"items": [...], "pagination": {...}}.
// @Security BearerAuth
// @Param ids query string false "comma-separated workspace IDs"
// @Param name query string false "workspace name"
// @Param alias query string false "workspace alias"
// @Param user_id query string false "user ID (optionally paginated)"
// @Param page query int false "page (default 1)"
// @Param page_size query int false "page size (default 50, max 100)"
// @Produce json
// @Success 200 {array} httpmodel.WorkspaceResponse "array for ids/user_id; a single object for name/alias; a paginated object when paginating by user_id (see description)"
// @Router /api/workspaces [get]
func (h *WorkspaceHandler) List(c echo.Context) error {
	ctx := c.Request().Context()
	uc := httpinternal.Usecases(c).Workspace
	op := httpinternal.Operator(c)

	ids := c.QueryParam("ids")
	name := c.QueryParam("name")
	alias := c.QueryParam("alias")
	userID := c.QueryParam("user_id")

	selectors := 0
	for _, s := range []string{ids, name, alias, userID} {
		if s != "" {
			selectors++
		}
	}
	if selectors != 1 {
		return badRequest("exactly one of ids, name, alias, user_id is required")
	}

	switch {
	case ids != "":
		wids, err := parseWorkspaceIDs(strings.Split(ids, ","))
		if err != nil {
			return err
		}
		ws, err := uc.Fetch(ctx, wids, op)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, httpmodel.NewWorkspaceResponses(ws))
	case name != "":
		w, err := uc.FetchByName(ctx, name)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, httpmodel.NewWorkspaceResponse(w))
	case alias != "":
		w, err := uc.FetchByAlias(ctx, alias)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, httpmodel.NewWorkspaceResponse(w))
	default: // user_id
		uid, err := id.UserIDFrom(userID)
		if err != nil {
			return badRequest("invalid user_id")
		}
		if c.QueryParam("page") != "" || c.QueryParam("page_size") != "" {
			var pp httpinternal.PageParams
			if err := c.Bind(&pp); err != nil {
				return err
			}
			page, size := pp.Normalized()
			res, err := uc.FetchByUserWithPagination(ctx, uid, interfaces.FetchByUserWithPaginationParam{Page: int64(page), Size: int64(size)})
			if err != nil {
				return err
			}
			return c.JSON(http.StatusOK, httpinternal.NewPageResult(httpmodel.NewWorkspaceResponses(res.Workspaces), page, size, res.TotalCount))
		}
		ws, err := uc.FindByUser(ctx, uid, op)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, httpmodel.NewWorkspaceResponses(ws))
	}
}

// Update godoc
// @Tags Workspace
// @Summary Update a workspace
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "workspace ID"
// @Param body body httpmodel.UpdateWorkspaceRequest true "fields to update"
// @Success 200 {object} httpmodel.WorkspaceResponse
// @Router /api/workspaces/{id} [patch]
func (h *WorkspaceHandler) Update(c echo.Context) error {
	ctx := c.Request().Context()
	wid, err := id.WorkspaceIDFrom(c.Param("id"))
	if err != nil {
		return badRequest("invalid workspace id")
	}
	req := &httpmodel.UpdateWorkspaceRequest{}
	if err := httpinternal.BindValidate(c, req); err != nil {
		return err
	}
	w, err := httpinternal.Usecases(c).Workspace.Update(ctx, interfaces.UpdateWorkspaceParam{
		ID: wid, Name: req.Name, Alias: req.Alias, Description: req.Description, Website: req.Website, PhotoURL: req.PhotoURL,
	}, httpinternal.Operator(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpmodel.NewWorkspaceResponse(w))
}

// Delete godoc
// @Tags Workspace
// @Summary Delete a workspace
// @Security BearerAuth
// @Param id path string true "workspace ID"
// @Success 204
// @Router /api/workspaces/{id} [delete]
func (h *WorkspaceHandler) Delete(c echo.Context) error {
	ctx := c.Request().Context()
	wid, err := id.WorkspaceIDFrom(c.Param("id"))
	if err != nil {
		return badRequest("invalid workspace id")
	}
	if err := httpinternal.Usecases(c).Workspace.Remove(ctx, wid, httpinternal.Operator(c)); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// AddMembers godoc
// @Tags Workspace
// @Summary Add user members to a workspace
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "workspace ID"
// @Param body body httpmodel.AddMembersRequest true "users to add"
// @Success 200 {object} httpmodel.WorkspaceResponse
// @Router /api/workspaces/{id}/members [post]
func (h *WorkspaceHandler) AddMembers(c echo.Context) error {
	ctx := c.Request().Context()
	wid, err := id.WorkspaceIDFrom(c.Param("id"))
	if err != nil {
		return badRequest("invalid workspace id")
	}
	req := &httpmodel.AddMembersRequest{}
	if err := httpinternal.BindValidate(c, req); err != nil {
		return err
	}
	m, err := req.BuildUserRoleMap()
	if err != nil {
		return err
	}
	w, err := httpinternal.Usecases(c).Workspace.AddUserMember(ctx, wid, m, httpinternal.Operator(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpmodel.NewWorkspaceResponse(w))
}

// UpdateMember godoc
// @Tags Workspace
// @Summary Update a user member's role
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "workspace ID"
// @Param user_id path string true "user ID"
// @Param body body httpmodel.UpdateMemberRequest true "new role"
// @Success 200 {object} httpmodel.WorkspaceResponse
// @Router /api/workspaces/{id}/members/{user_id} [patch]
func (h *WorkspaceHandler) UpdateMember(c echo.Context) error {
	ctx := c.Request().Context()
	wid, err := id.WorkspaceIDFrom(c.Param("id"))
	if err != nil {
		return badRequest("invalid workspace id")
	}
	uid, err := id.UserIDFrom(c.Param("user_id"))
	if err != nil {
		return badRequest("invalid user id")
	}
	req := &httpmodel.UpdateMemberRequest{}
	if err := httpinternal.BindValidate(c, req); err != nil {
		return err
	}
	w, err := httpinternal.Usecases(c).Workspace.UpdateUserMember(ctx, wid, uid, httpmodel.ParseRole(req.Role), httpinternal.Operator(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpmodel.NewWorkspaceResponse(w))
}

// RemoveMember godoc
// @Tags Workspace
// @Summary Remove a user member
// @Security BearerAuth
// @Param id path string true "workspace ID"
// @Param user_id path string true "user ID"
// @Produce json
// @Success 200 {object} httpmodel.WorkspaceResponse
// @Router /api/workspaces/{id}/members/{user_id} [delete]
func (h *WorkspaceHandler) RemoveMember(c echo.Context) error {
	ctx := c.Request().Context()
	wid, err := id.WorkspaceIDFrom(c.Param("id"))
	if err != nil {
		return badRequest("invalid workspace id")
	}
	uid, err := id.UserIDFrom(c.Param("user_id"))
	if err != nil {
		return badRequest("invalid user id")
	}
	w, err := httpinternal.Usecases(c).Workspace.RemoveUserMember(ctx, wid, uid, httpinternal.Operator(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpmodel.NewWorkspaceResponse(w))
}

// RemoveMembers godoc
// @Tags Workspace
// @Summary Remove multiple user members
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "workspace ID"
// @Param body body httpmodel.RemoveMembersRequest true "user IDs"
// @Success 200 {object} httpmodel.WorkspaceResponse
// @Router /api/workspaces/{id}/members [delete]
func (h *WorkspaceHandler) RemoveMembers(c echo.Context) error {
	ctx := c.Request().Context()
	wid, err := id.WorkspaceIDFrom(c.Param("id"))
	if err != nil {
		return badRequest("invalid workspace id")
	}
	req := &httpmodel.RemoveMembersRequest{}
	if err := httpinternal.BindValidate(c, req); err != nil {
		return err
	}
	uids, err := parseUserIDs(req.UserIDs)
	if err != nil {
		return err
	}
	w, err := httpinternal.Usecases(c).Workspace.RemoveMultipleUserMembers(ctx, wid, uids, httpinternal.Operator(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpmodel.NewWorkspaceResponse(w))
}

// AddIntegration godoc
// @Tags Workspace
// @Summary Add an integration member
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "workspace ID"
// @Param body body httpmodel.AddIntegrationRequest true "integration and role"
// @Success 200 {object} httpmodel.WorkspaceResponse
// @Router /api/workspaces/{id}/integrations [post]
func (h *WorkspaceHandler) AddIntegration(c echo.Context) error {
	ctx := c.Request().Context()
	wid, err := id.WorkspaceIDFrom(c.Param("id"))
	if err != nil {
		return badRequest("invalid workspace id")
	}
	req := &httpmodel.AddIntegrationRequest{}
	if err := httpinternal.BindValidate(c, req); err != nil {
		return err
	}
	iid, err := id.IntegrationIDFrom(req.IntegrationID)
	if err != nil {
		return badRequest("invalid integration id")
	}
	w, err := httpinternal.Usecases(c).Workspace.AddIntegrationMember(ctx, wid, iid, httpmodel.ParseRole(req.Role), httpinternal.Operator(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpmodel.NewWorkspaceResponse(w))
}

// UpdateIntegration godoc
// @Tags Workspace
// @Summary Update an integration member's role
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "workspace ID"
// @Param integration_id path string true "integration ID"
// @Param body body httpmodel.UpdateIntegrationRequest true "new role"
// @Success 200 {object} httpmodel.WorkspaceResponse
// @Router /api/workspaces/{id}/integrations/{integration_id} [patch]
func (h *WorkspaceHandler) UpdateIntegration(c echo.Context) error {
	ctx := c.Request().Context()
	wid, err := id.WorkspaceIDFrom(c.Param("id"))
	if err != nil {
		return badRequest("invalid workspace id")
	}
	iid, err := id.IntegrationIDFrom(c.Param("integration_id"))
	if err != nil {
		return badRequest("invalid integration id")
	}
	req := &httpmodel.UpdateIntegrationRequest{}
	if err := httpinternal.BindValidate(c, req); err != nil {
		return err
	}
	w, err := httpinternal.Usecases(c).Workspace.UpdateIntegration(ctx, wid, iid, httpmodel.ParseRole(req.Role), httpinternal.Operator(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpmodel.NewWorkspaceResponse(w))
}

// RemoveIntegration godoc
// @Tags Workspace
// @Summary Remove an integration member
// @Security BearerAuth
// @Param id path string true "workspace ID"
// @Param integration_id path string true "integration ID"
// @Produce json
// @Success 200 {object} httpmodel.WorkspaceResponse
// @Router /api/workspaces/{id}/integrations/{integration_id} [delete]
func (h *WorkspaceHandler) RemoveIntegration(c echo.Context) error {
	ctx := c.Request().Context()
	wid, err := id.WorkspaceIDFrom(c.Param("id"))
	if err != nil {
		return badRequest("invalid workspace id")
	}
	iid, err := id.IntegrationIDFrom(c.Param("integration_id"))
	if err != nil {
		return badRequest("invalid integration id")
	}
	w, err := httpinternal.Usecases(c).Workspace.RemoveIntegration(ctx, wid, iid, httpinternal.Operator(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpmodel.NewWorkspaceResponse(w))
}

// RemoveIntegrations godoc
// @Tags Workspace
// @Summary Remove multiple integration members
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "workspace ID"
// @Param body body httpmodel.RemoveIntegrationsRequest true "integration IDs"
// @Success 200 {object} httpmodel.WorkspaceResponse
// @Router /api/workspaces/{id}/integrations [delete]
func (h *WorkspaceHandler) RemoveIntegrations(c echo.Context) error {
	ctx := c.Request().Context()
	wid, err := id.WorkspaceIDFrom(c.Param("id"))
	if err != nil {
		return badRequest("invalid workspace id")
	}
	req := &httpmodel.RemoveIntegrationsRequest{}
	if err := httpinternal.BindValidate(c, req); err != nil {
		return err
	}
	iids := make(id.IntegrationIDList, 0, len(req.IntegrationIDs))
	for _, s := range req.IntegrationIDs {
		iid, err := id.IntegrationIDFrom(s)
		if err != nil {
			return badRequest("invalid integration id")
		}
		iids = append(iids, iid)
	}
	w, err := httpinternal.Usecases(c).Workspace.RemoveIntegrations(ctx, wid, iids, httpinternal.Operator(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpmodel.NewWorkspaceResponse(w))
}

// TransferOwnership godoc
// @Tags Workspace
// @Summary Transfer workspace ownership
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "workspace ID"
// @Param body body httpmodel.TransferOwnershipRequest true "new owner ID"
// @Success 200 {object} httpmodel.WorkspaceResponse
// @Router /api/workspaces/{id}/transfer-ownership [post]
func (h *WorkspaceHandler) TransferOwnership(c echo.Context) error {
	ctx := c.Request().Context()
	wid, err := id.WorkspaceIDFrom(c.Param("id"))
	if err != nil {
		return badRequest("invalid workspace id")
	}
	req := &httpmodel.TransferOwnershipRequest{}
	if err := httpinternal.BindValidate(c, req); err != nil {
		return err
	}
	newOwner, err := id.UserIDFrom(req.NewOwnerID)
	if err != nil {
		return badRequest("invalid new_owner_id")
	}
	w, err := httpinternal.Usecases(c).Workspace.TransferOwnership(ctx, wid, newOwner, httpinternal.Operator(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpmodel.NewWorkspaceResponse(w))
}

func parseWorkspaceIDs(ss []string) (id.WorkspaceIDList, error) {
	out := make(id.WorkspaceIDList, 0, len(ss))
	for _, s := range ss {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		wid, err := id.WorkspaceIDFrom(s)
		if err != nil {
			return nil, badRequest("invalid workspace id")
		}
		out = append(out, wid)
	}
	return out, nil
}

func parseUserIDs(ss []string) (id.UserIDList, error) {
	out := make(id.UserIDList, 0, len(ss))
	for _, s := range ss {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		uid, err := id.UserIDFrom(s)
		if err != nil {
			return nil, badRequest("invalid user id")
		}
		out = append(out, uid)
	}
	return out, nil
}
