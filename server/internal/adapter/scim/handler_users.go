package scim

import (
	"context"
	"encoding/json"
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

// UserHandler handles SCIM 2.0 /scim/v2/Users routes.
type UserHandler struct {
	baseURL       string
	scimUC        interfaces.Scim
	workspaceRepo workspace.Repo
}

// NewUserHandler constructs a UserHandler.
func NewUserHandler(scimUC interfaces.Scim, workspaceRepo workspace.Repo, baseURL string) *UserHandler {
	return &UserHandler{
		baseURL:       baseURL,
		scimUC:        scimUC,
		workspaceRepo: workspaceRepo,
	}
}

// Create handles POST /scim/v2/Users — provision a new user (201 Created).
func (h *UserHandler) Create(c echo.Context) error {
	ctx := c.Request().Context()

	wsID, ok := WorkspaceIDFromContext(ctx)
	if !ok {
		return scimErrorResponse(c, http.StatusUnauthorized, "workspace not resolved", "")
	}

	var body ScimUser
	if err := c.Bind(&body); err != nil {
		return scimErrorResponse(c, http.StatusBadRequest, "invalid request body", "invalidValue")
	}

	email := body.UserName
	if email == "" && len(body.Emails) > 0 {
		email = body.Emails[0].Value
	}
	name := body.Name.Formatted
	if name == "" {
		name = email
	}

	u, err := h.scimUC.ProvisionScimUser(ctx, interfaces.ProvisionScimUserParam{
		Email:       email,
		ExternalID:  body.ExternalID,
		Name:        name,
		Role:        role.RoleReader,
		WorkspaceID: wsID,
	})
	if err != nil {
		return h.mapError(c, err)
	}

	member := h.memberForUser(ctx, wsID, u.ID())
	resp := DomainUserToScimUser(u, member, h.baseURL)

	c.Response().Header().Set("Location", resp.Meta.Location)
	return c.JSON(http.StatusCreated, resp)
}

// Delete handles DELETE /scim/v2/Users/:id — soft-deprovision (200 OK with active:false).
func (h *UserHandler) Delete(c echo.Context) error {
	ctx := c.Request().Context()

	wsID, ok := WorkspaceIDFromContext(ctx)
	if !ok {
		return scimErrorResponse(c, http.StatusUnauthorized, "workspace not resolved", "")
	}

	uid, err := user.IDFrom(c.Param("id"))
	if err != nil {
		return scimErrorResponse(c, http.StatusNotFound, "user not found", "")
	}

	// Retrieve user first to build the response.
	u, err := h.scimUC.GetScimUser(ctx, wsID, uid)
	if err != nil {
		return h.mapError(c, err)
	}

	memberBefore := h.memberForUser(ctx, wsID, uid)

	if err := h.scimUC.DeprovisionScimUserByUserID(ctx, wsID, uid); err != nil {
		return h.mapError(c, err)
	}

	// Build disabled member for the response to reflect active:false.
	disabledMember := memberBefore
	disabledMember.Disabled = true
	resp := DomainUserToScimUser(u, disabledMember, h.baseURL)
	return c.JSON(http.StatusOK, resp)
}

// Get handles GET /scim/v2/Users/:id.
func (h *UserHandler) Get(c echo.Context) error {
	ctx := c.Request().Context()

	wsID, ok := WorkspaceIDFromContext(ctx)
	if !ok {
		return scimErrorResponse(c, http.StatusUnauthorized, "workspace not resolved", "")
	}

	uid, err := user.IDFrom(c.Param("id"))
	if err != nil {
		return scimErrorResponse(c, http.StatusNotFound, "user not found", "")
	}

	u, err := h.scimUC.GetScimUser(ctx, wsID, uid)
	if err != nil {
		return h.mapError(c, err)
	}

	member := h.memberForUser(ctx, wsID, uid)
	return c.JSON(http.StatusOK, DomainUserToScimUser(u, member, h.baseURL))
}

// List handles GET /scim/v2/Users with optional ?filter= query param.
func (h *UserHandler) List(c echo.Context) error {
	ctx := c.Request().Context()

	wsID, ok := WorkspaceIDFromContext(ctx)
	if !ok {
		return scimErrorResponse(c, http.StatusUnauthorized, "workspace not resolved", "")
	}

	filterParam := c.QueryParam("filter")

	users, err := h.scimUC.ListScimUsers(ctx, wsID, filterParam)
	if err != nil {
		return h.mapError(c, err)
	}

	// Apply in-memory filter if provided.
	var filtered []*user.User
	if filterParam == "" {
		filtered = users
	} else {
		attr, op, val, parseErr := parseFilter(filterParam)
		if parseErr != nil {
			return scimErrorResponse(c, http.StatusBadRequest, "unsupported filter expression", "invalidFilter")
		}
		switch {
		case attr == "username" && op == "eq":
			for _, u := range users {
				if strings.EqualFold(u.Email(), val) {
					filtered = append(filtered, u)
				}
			}
		case attr == "externalid" && op == "eq":
			members := h.membersForWorkspace(ctx, wsID)
			for _, u := range users {
				if m, ok := members[u.ID()]; ok && m.ExternalID == val {
					filtered = append(filtered, u)
				}
			}
		default:
			return scimErrorResponse(c, http.StatusBadRequest, "unsupported filter attribute", "invalidFilter")
		}
	}

	resources := make([]ScimUser, 0, len(filtered))
	members := h.membersForWorkspace(ctx, wsID)
	for _, u := range filtered {
		member := members[u.ID()]
		resources = append(resources, DomainUserToScimUser(u, member, h.baseURL))
	}

	return c.JSON(http.StatusOK, ScimListResponse{
		ItemsPerPage: len(resources),
		Resources:    resources,
		Schemas:      []string{ScimSchemaListResponse},
		StartIndex:   1,
		TotalResults: len(resources),
	})
}

// Patch handles PATCH /scim/v2/Users/:id — partial update (handles deprovisioning).
func (h *UserHandler) Patch(c echo.Context) error {
	ctx := c.Request().Context()

	wsID, ok := WorkspaceIDFromContext(ctx)
	if !ok {
		return scimErrorResponse(c, http.StatusUnauthorized, "workspace not resolved", "")
	}

	uid, err := user.IDFrom(c.Param("id"))
	if err != nil {
		return scimErrorResponse(c, http.StatusNotFound, "user not found", "")
	}

	var patchOp ScimPatchOp
	if err := c.Bind(&patchOp); err != nil {
		return scimErrorResponse(c, http.StatusBadRequest, "invalid request body", "invalidValue")
	}

	// Retrieve user to confirm existence.
	u, err := h.scimUC.GetScimUser(ctx, wsID, uid)
	if err != nil {
		return h.mapError(c, err)
	}

	deprovisioned := false
	for _, op := range patchOp.Operations {
		if !strings.EqualFold(op.Op, "replace") {
			continue
		}

		// Okta format: {"op":"replace","path":"active","value":false}
		if strings.EqualFold(op.Path, "active") {
			if active, ok := parseBoolValue(op.Value); ok && !active {
				deprovisioned = true
			}
			continue
		}

		// Azure AD format: {"op":"replace","value":{"active":false}}
		if op.Path == "" {
			if active, ok := extractActiveBool(op.Value); ok && !active {
				deprovisioned = true
			}
		}
	}

	if deprovisioned {
		if err := h.scimUC.DeprovisionScimUserByUserID(ctx, wsID, uid); err != nil {
			return h.mapError(c, err)
		}
	}

	member := h.memberForUser(ctx, wsID, uid)
	return c.JSON(http.StatusOK, DomainUserToScimUser(u, member, h.baseURL))
}

// Replace handles PUT /scim/v2/Users/:id — full replace (returns current state).
func (h *UserHandler) Replace(c echo.Context) error {
	ctx := c.Request().Context()

	wsID, ok := WorkspaceIDFromContext(ctx)
	if !ok {
		return scimErrorResponse(c, http.StatusUnauthorized, "workspace not resolved", "")
	}

	uid, err := user.IDFrom(c.Param("id"))
	if err != nil {
		return scimErrorResponse(c, http.StatusNotFound, "user not found", "")
	}

	u, err := h.scimUC.GetScimUser(ctx, wsID, uid)
	if err != nil {
		return h.mapError(c, err)
	}

	member := h.memberForUser(ctx, wsID, uid)
	return c.JSON(http.StatusOK, DomainUserToScimUser(u, member, h.baseURL))
}

// --- helpers ---

// memberForUser retrieves the workspace.Member for a given user within a workspace.
// Returns an empty Member on any error (graceful degradation).
func (h *UserHandler) memberForUser(ctx context.Context, wsID workspace.ID, uid user.ID) workspace.Member {
	ws, err := h.workspaceRepo.FindByID(ctx, wsID)
	if err != nil {
		return workspace.Member{}
	}
	if m := ws.Members().User(uid); m != nil {
		return *m
	}
	return workspace.Member{}
}

// membersForWorkspace returns all members of a workspace as a map.
// Returns an empty map on error.
func (h *UserHandler) membersForWorkspace(ctx context.Context, wsID workspace.ID) map[workspace.UserID]workspace.Member {
	ws, err := h.workspaceRepo.FindByID(ctx, wsID)
	if err != nil {
		return map[workspace.UserID]workspace.Member{}
	}
	return ws.Members().Users()
}

// mapError converts domain errors to appropriate SCIM HTTP responses.
func (h *UserHandler) mapError(c echo.Context, err error) error {
	if errors.Is(err, rerror.ErrNotFound) || errors.Is(err, interfaces.ErrSCIMUserNotFound) {
		return scimErrorResponse(c, http.StatusNotFound, "user not found", "")
	}
	if errors.Is(err, interfaces.ErrOwnerCannotLeaveTheWorkspace) {
		return scimErrorResponse(c, http.StatusConflict, "cannot deprovision the last owner", "")
	}
	if errors.Is(err, interfaces.ErrSCIMNotEnabled) {
		return scimErrorResponse(c, http.StatusForbidden, "SCIM is not enabled for this workspace", "")
	}
	return scimErrorResponse(c, http.StatusInternalServerError, "internal server error", "")
}

// parseFilter parses a simple SCIM filter of the form `attr eq "value"`.
// Returns the lower-cased attribute name, operator, unquoted value, and any error.
func parseFilter(filter string) (attr, op, val string, err error) {
	parts := strings.SplitN(strings.TrimSpace(filter), " ", 3)
	if len(parts) != 3 {
		return "", "", "", errors.New("invalid filter syntax")
	}
	attr = strings.ToLower(parts[0])
	op = strings.ToLower(parts[1])
	val = strings.Trim(parts[2], `"`)
	if op != "eq" {
		return "", "", "", errors.New("only eq operator is supported")
	}
	return attr, op, val, nil
}

// parseBoolValue attempts to extract a boolean from an interface{} value.
func parseBoolValue(v interface{}) (bool, bool) {
	switch t := v.(type) {
	case bool:
		return t, true
	case float64:
		return t != 0, true
	}
	return false, false
}

// extractActiveBool extracts the "active" field from a map/object value (Azure AD PATCH format).
func extractActiveBool(v interface{}) (bool, bool) {
	if m, ok := v.(map[string]interface{}); ok {
		if av, ok := m["active"]; ok {
			return parseBoolValue(av)
		}
		return false, false
	}
	// Try JSON round-trip for edge cases where the value is already serialised.
	data, err := json.Marshal(v)
	if err != nil {
		return false, false
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return false, false
	}
	if av, ok := obj["active"]; ok {
		return parseBoolValue(av)
	}
	return false, false
}
