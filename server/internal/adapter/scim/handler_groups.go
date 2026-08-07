package scim

import (
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/rerror"
)

// GroupHandler handles SCIM 2.0 /scim/v2/Groups routes.
type GroupHandler struct {
	baseURL       string
	scimUC        interfaces.Scim
	workspaceRepo workspace.Repo
}

// NewGroupHandler constructs a GroupHandler.
func NewGroupHandler(scimUC interfaces.Scim, workspaceRepo workspace.Repo, baseURL string) *GroupHandler {
	return &GroupHandler{
		baseURL:       baseURL,
		scimUC:        scimUC,
		workspaceRepo: workspaceRepo,
	}
}

// Create handles POST /scim/v2/Groups — ensure group→role mapping exists, upsert members.
func (h *GroupHandler) Create(c echo.Context) error {
	ctx := c.Request().Context()
	wsID, ok := WorkspaceIDFromContext(ctx)
	if !ok {
		return scimErrorResponse(c, http.StatusUnauthorized, "workspace not resolved", "")
	}

	var body ScimGroup
	if err := c.Bind(&body); err != nil {
		return scimErrorResponse(c, http.StatusBadRequest, "invalid request body", "invalidValue")
	}

	members := h.wireToInterfaceMembers(body.Members)
	if err := h.scimUC.SyncScimGroup(ctx, wsID, "", body.DisplayName, members); err != nil {
		return h.mapError(c, err)
	}

	ws, err := h.workspaceRepo.FindByID(ctx, wsID)
	if err != nil {
		return scimErrorResponse(c, http.StatusInternalServerError, "internal server error", "")
	}

	group := h.buildGroup(ws, wsID, body.DisplayName)
	c.Response().Header().Set("Location", group.Meta.Location)
	return c.JSON(http.StatusCreated, group)
}

// Delete handles DELETE /scim/v2/Groups/:id — remove the group mapping (no deprovisioning).
func (h *GroupHandler) Delete(c echo.Context) error {
	ctx := c.Request().Context()
	wsID, ok := WorkspaceIDFromContext(ctx)
	if !ok {
		return scimErrorResponse(c, http.StatusUnauthorized, "workspace not resolved", "")
	}

	groupID := c.Param("id")
	_, groupName, err := parseGroupID(groupID)
	if err != nil {
		return scimErrorResponse(c, http.StatusNotFound, "group not found", "")
	}

	if err := h.scimUC.DeleteScimGroup(ctx, wsID, groupName); err != nil {
		return h.mapError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

// Get handles GET /scim/v2/Groups/:id.
func (h *GroupHandler) Get(c echo.Context) error {
	ctx := c.Request().Context()
	wsID, ok := WorkspaceIDFromContext(ctx)
	if !ok {
		return scimErrorResponse(c, http.StatusUnauthorized, "workspace not resolved", "")
	}

	groupID := c.Param("id")
	_, groupName, err := parseGroupID(groupID)
	if err != nil {
		return scimErrorResponse(c, http.StatusNotFound, "group not found", "")
	}

	ws, err := h.workspaceRepo.FindByID(ctx, wsID)
	if err != nil {
		return scimErrorResponse(c, http.StatusInternalServerError, "internal server error", "")
	}

	return c.JSON(http.StatusOK, h.buildGroup(ws, wsID, groupName))
}

// List handles GET /scim/v2/Groups — returns one group per occupied role bucket.
func (h *GroupHandler) List(c echo.Context) error {
	ctx := c.Request().Context()
	wsID, ok := WorkspaceIDFromContext(ctx)
	if !ok {
		return scimErrorResponse(c, http.StatusUnauthorized, "workspace not resolved", "")
	}

	ws, err := h.workspaceRepo.FindByID(ctx, wsID)
	if err != nil {
		return scimErrorResponse(c, http.StatusInternalServerError, "internal server error", "")
	}

	groups := h.buildGroupList(ws)
	return c.JSON(http.StatusOK, ScimListResponse{
		ItemsPerPage: len(groups),
		Resources:    groups,
		Schemas:      []string{ScimSchemaListResponse},
		StartIndex:   1,
		TotalResults: len(groups),
	})
}

// Patch handles PATCH /scim/v2/Groups/:id — add/remove members or rename group.
// Okta format: {"op":"add","path":"members","value":[{...}]}
// Both add and remove ops translate to SyncScimGroup calls.
func (h *GroupHandler) Patch(c echo.Context) error {
	ctx := c.Request().Context()
	wsID, ok := WorkspaceIDFromContext(ctx)
	if !ok {
		return scimErrorResponse(c, http.StatusUnauthorized, "workspace not resolved", "")
	}

	groupID := c.Param("id")
	_, groupName, err := parseGroupID(groupID)
	if err != nil {
		return scimErrorResponse(c, http.StatusNotFound, "group not found", "")
	}

	var patchOp ScimPatchOp
	if err := c.Bind(&patchOp); err != nil {
		return scimErrorResponse(c, http.StatusBadRequest, "invalid request body", "invalidValue")
	}

	ws, err := h.workspaceRepo.FindByID(ctx, wsID)
	if err != nil {
		return scimErrorResponse(c, http.StatusInternalServerError, "internal server error", "")
	}

	// Build current member list for this group so we can apply deltas.
	currentMembers := h.currentGroupMembers(ws, groupName)

	for _, op := range patchOp.Operations {
		opLower := strings.ToLower(op.Op)
		pathLower := strings.ToLower(op.Path)

		switch {
		case opLower == "add" && pathLower == "members":
			added := h.extractInterfaceMembers(op.Value)
			currentMembers = h.mergeMembers(currentMembers, added)
		case opLower == "remove" && pathLower == "members":
			removed := h.extractInterfaceMembers(op.Value)
			currentMembers = h.subtractMembers(currentMembers, removed)
		case opLower == "replace" && pathLower == "displayname":
			if name, ok := op.Value.(string); ok && name != "" {
				groupName = name
			}
		}
	}

	if err := h.scimUC.SyncScimGroup(ctx, wsID, groupID, groupName, currentMembers); err != nil {
		return h.mapError(c, err)
	}

	ws, err = h.workspaceRepo.FindByID(ctx, wsID)
	if err != nil {
		return scimErrorResponse(c, http.StatusInternalServerError, "internal server error", "")
	}

	return c.JSON(http.StatusOK, h.buildGroup(ws, wsID, groupName))
}

// Replace handles PUT /scim/v2/Groups/:id — full replace of all group members.
func (h *GroupHandler) Replace(c echo.Context) error {
	ctx := c.Request().Context()
	wsID, ok := WorkspaceIDFromContext(ctx)
	if !ok {
		return scimErrorResponse(c, http.StatusUnauthorized, "workspace not resolved", "")
	}

	groupID := c.Param("id")
	_, groupName, err := parseGroupID(groupID)
	if err != nil {
		return scimErrorResponse(c, http.StatusNotFound, "group not found", "")
	}

	var body ScimGroup
	if err := c.Bind(&body); err != nil {
		return scimErrorResponse(c, http.StatusBadRequest, "invalid request body", "invalidValue")
	}

	members := h.wireToInterfaceMembers(body.Members)
	if err := h.scimUC.SyncScimGroup(ctx, wsID, groupID, groupName, members); err != nil {
		return h.mapError(c, err)
	}

	ws, err := h.workspaceRepo.FindByID(ctx, wsID)
	if err != nil {
		return scimErrorResponse(c, http.StatusInternalServerError, "internal server error", "")
	}

	return c.JSON(http.StatusOK, h.buildGroup(ws, wsID, groupName))
}

// --- helpers ---

// buildGroupList returns one ScimGroup per role bucket that has at least one active member.
func (h *GroupHandler) buildGroupList(ws *workspace.Workspace) []ScimGroup {
	members := ws.Members().Users()
	if len(members) == 0 {
		return []ScimGroup{}
	}

	// Reverse the groupRoleMapping: role → group name.
	roleToGroup := make(map[role.RoleType]string)
	if cfg := ws.ScimConfig(); cfg != nil {
		for gName, r := range cfg.GroupRoleMapping() {
			roleToGroup[r] = gName
		}
	}

	type entry struct {
		name    string
		members []ScimGroupMember
	}
	buckets := make(map[role.RoleType]*entry)

	wsID := ws.ID()
	for uid, m := range members {
		if m.Disabled {
			continue
		}
		b, exists := buckets[m.Role]
		if !exists {
			gName, ok := roleToGroup[m.Role]
			if !ok {
				gName = string(m.Role)
			}
			b = &entry{name: gName}
			buckets[m.Role] = b
		}
		b.members = append(b.members, ScimGroupMember{Value: uid.String()})
	}

	groups := make([]ScimGroup, 0, len(buckets))
	for _, b := range buckets {
		gid := makeGroupID(wsID, b.name)
		groups = append(groups, ScimGroup{
			DisplayName: b.name,
			ID:          gid,
			Members:     b.members,
			Meta: ScimMeta{
				Location:     h.baseURL + "/scim/v2/Groups/" + gid,
				ResourceType: "Group",
			},
			Schemas: []string{ScimSchemaGroup},
		})
	}
	return groups
}

// buildGroup constructs a ScimGroup for the given group name within a workspace.
// The group's role is resolved via GroupRoleMapping; unmapped names default to RoleReader.
func (h *GroupHandler) buildGroup(ws *workspace.Workspace, wsID workspace.ID, groupName string) ScimGroup {
	groupRole := role.RoleReader
	if cfg := ws.ScimConfig(); cfg != nil {
		if mapping := cfg.GroupRoleMapping(); mapping != nil {
			if r, ok := mapping[groupName]; ok {
				groupRole = r
			}
		}
	}

	var scimMembers []ScimGroupMember
	for uid, m := range ws.Members().Users() {
		if !m.Disabled && m.Role == groupRole {
			scimMembers = append(scimMembers, ScimGroupMember{Value: uid.String()})
		}
	}

	gid := makeGroupID(wsID, groupName)
	return ScimGroup{
		DisplayName: groupName,
		ID:          gid,
		Members:     scimMembers,
		Meta: ScimMeta{
			Location:     h.baseURL + "/scim/v2/Groups/" + gid,
			ResourceType: "Group",
		},
		Schemas: []string{ScimSchemaGroup},
	}
}

// currentGroupMembers returns all workspace members belonging to the given group's role bucket.
func (h *GroupHandler) currentGroupMembers(ws *workspace.Workspace, groupName string) []interfaces.ScimGroupMember {
	groupRole := role.RoleReader
	if cfg := ws.ScimConfig(); cfg != nil {
		if mapping := cfg.GroupRoleMapping(); mapping != nil {
			if r, ok := mapping[groupName]; ok {
				groupRole = r
			}
		}
	}

	var out []interfaces.ScimGroupMember
	for uid, m := range ws.Members().Users() {
		if m.Role == groupRole {
			uid := uid
			out = append(out, interfaces.ScimGroupMember{
				ExternalID: m.ExternalID,
				UserID:     &uid,
			})
		}
	}
	return out
}

// wireToInterfaceMembers converts SCIM wire members (Value = user ID string) to usecase members.
func (h *GroupHandler) wireToInterfaceMembers(wire []ScimGroupMember) []interfaces.ScimGroupMember {
	out := make([]interfaces.ScimGroupMember, 0, len(wire))
	for _, m := range wire {
		uid, err := user.IDFrom(m.Value)
		if err != nil {
			continue
		}
		out = append(out, interfaces.ScimGroupMember{UserID: &uid})
	}
	return out
}

// extractInterfaceMembers parses a PATCH op value into usecase members.
// Supports []interface{} where each item is map[string]interface{} with a "value" key.
func (h *GroupHandler) extractInterfaceMembers(v interface{}) []interfaces.ScimGroupMember {
	list, ok := v.([]interface{})
	if !ok {
		return nil
	}
	var out []interfaces.ScimGroupMember
	for _, item := range list {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		val, _ := m["value"].(string)
		if val == "" {
			continue
		}
		uid, err := user.IDFrom(val)
		if err != nil {
			continue
		}
		out = append(out, interfaces.ScimGroupMember{UserID: &uid})
	}
	return out
}

// mergeMembers appends newMembers to existing without duplicates (by UserID).
func (h *GroupHandler) mergeMembers(existing, newMembers []interfaces.ScimGroupMember) []interfaces.ScimGroupMember {
	seen := make(map[user.ID]struct{}, len(existing))
	for _, m := range existing {
		if m.UserID != nil {
			seen[*m.UserID] = struct{}{}
		}
	}
	result := append([]interfaces.ScimGroupMember(nil), existing...)
	for _, m := range newMembers {
		if m.UserID == nil {
			continue
		}
		if _, ok := seen[*m.UserID]; !ok {
			seen[*m.UserID] = struct{}{}
			result = append(result, m)
		}
	}
	return result
}

// subtractMembers removes toRemove entries from existing (matched by UserID).
func (h *GroupHandler) subtractMembers(existing, toRemove []interfaces.ScimGroupMember) []interfaces.ScimGroupMember {
	removeSet := make(map[user.ID]struct{}, len(toRemove))
	for _, m := range toRemove {
		if m.UserID != nil {
			removeSet[*m.UserID] = struct{}{}
		}
	}
	var result []interfaces.ScimGroupMember
	for _, m := range existing {
		if m.UserID != nil {
			if _, skip := removeSet[*m.UserID]; skip {
				continue
			}
		}
		result = append(result, m)
	}
	return result
}

// mapError converts domain errors to SCIM HTTP error responses.
func (h *GroupHandler) mapError(c echo.Context, err error) error {
	if errors.Is(err, rerror.ErrNotFound) {
		return scimErrorResponse(c, http.StatusNotFound, "group not found", "")
	}
	if errors.Is(err, interfaces.ErrOwnerCannotLeaveTheWorkspace) {
		return scimErrorResponse(c, http.StatusConflict, "cannot remove the last owner", "")
	}
	if errors.Is(err, interfaces.ErrSCIMNotEnabled) {
		return scimErrorResponse(c, http.StatusForbidden, "SCIM is not enabled for this workspace", "")
	}
	return scimErrorResponse(c, http.StatusInternalServerError, "internal server error", "")
}
