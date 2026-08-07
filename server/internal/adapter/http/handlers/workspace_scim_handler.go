package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/adapter/http/httpmodel"
	httpinternal "github.com/reearth/reearth-accounts/server/internal/adapter/http/internal"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
)

// WorkspaceScimHandler handles admin REST endpoints for per-workspace SCIM configuration.
type WorkspaceScimHandler struct{}

func NewWorkspaceScimHandler() *WorkspaceScimHandler { return &WorkspaceScimHandler{} }

// GenerateToken handles POST /api/workspaces/:id/scim/token.
// Generates (or rotates) the SCIM bearer token for the workspace.
// The plaintext token is returned once and never stored; a second call rotates it.
func (h *WorkspaceScimHandler) GenerateToken(c echo.Context) error {
	ctx := c.Request().Context()
	wid, err := id.WorkspaceIDFrom(c.Param("id"))
	if err != nil {
		return httpinternal.NewError(http.StatusNotFound, "workspace not found", nil)
	}

	plaintext, err := httpinternal.Usecases(c).Scim.GenerateScimToken(ctx, wid, httpinternal.Operator(c))
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, httpmodel.GenerateScimTokenResponse{
		Token:   plaintext,
		Warning: "This token will not be shown again. Store it immediately.",
	})
}

// GetConfig handles GET /api/workspaces/:id/scim/config.
// Returns the current SCIM config; token_hash is masked as token_issued bool.
func (h *WorkspaceScimHandler) GetConfig(c echo.Context) error {
	ctx := c.Request().Context()
	wid, err := id.WorkspaceIDFrom(c.Param("id"))
	if err != nil {
		return httpinternal.NewError(http.StatusNotFound, "workspace not found", nil)
	}

	cfg, err := httpinternal.Usecases(c).Scim.GetScimConfig(ctx, wid, httpinternal.Operator(c))
	if err != nil {
		return err
	}

	scimBaseURL := c.Scheme() + "://" + c.Request().Host + "/scim/v2"
	return c.JSON(http.StatusOK, httpmodel.NewScimConfigResponse(cfg, scimBaseURL))
}

// RevokeToken handles DELETE /api/workspaces/:id/scim/token.
// Clears the stored token hash and disables SCIM for the workspace.
func (h *WorkspaceScimHandler) RevokeToken(c echo.Context) error {
	ctx := c.Request().Context()
	wid, err := id.WorkspaceIDFrom(c.Param("id"))
	if err != nil {
		return httpinternal.NewError(http.StatusNotFound, "workspace not found", nil)
	}

	if err := httpinternal.Usecases(c).Scim.RevokeScimToken(ctx, wid, httpinternal.Operator(c)); err != nil {
		return err
	}

	return c.NoContent(http.StatusNoContent)
}

// UpdateConfig handles PUT /api/workspaces/:id/scim/config.
// Enables/disables SCIM and sets the group→role mapping.
// Returns 400 if any role value is unrecognised.
func (h *WorkspaceScimHandler) UpdateConfig(c echo.Context) error {
	ctx := c.Request().Context()
	wid, err := id.WorkspaceIDFrom(c.Param("id"))
	if err != nil {
		return httpinternal.NewError(http.StatusNotFound, "workspace not found", nil)
	}

	req := &httpmodel.UpdateScimConfigRequest{}
	if err := httpinternal.BindValidate(c, req); err != nil {
		return err
	}

	mapping := make(map[string]role.RoleType, len(req.GroupRoleMapping))
	for groupName, roleStr := range req.GroupRoleMapping {
		r := role.RoleType(roleStr)
		if !r.Valid() {
			return httpinternal.NewError(http.StatusBadRequest, "invalid role: "+roleStr, nil)
		}
		mapping[groupName] = r
	}

	cfg, err := httpinternal.Usecases(c).Scim.UpdateScimConfig(ctx, wid, req.Enabled, mapping, httpinternal.Operator(c))
	if err != nil {
		return err
	}

	scimBaseURL := c.Scheme() + "://" + c.Request().Host + "/scim/v2"
	return c.JSON(http.StatusOK, httpmodel.NewScimConfigResponse(cfg, scimBaseURL))
}
