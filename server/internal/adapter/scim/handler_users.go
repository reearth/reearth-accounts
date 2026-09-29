package scim

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
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
	scimUC        interfaces.Scim
	workspaceRepo workspace.Repo
}

// NewUserHandler constructs a UserHandler.
func NewUserHandler(scimUC interfaces.Scim, workspaceRepo workspace.Repo) *UserHandler {
	return &UserHandler{
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

	var req ScimUserWriteRequest
	if err := c.Bind(&req); err != nil {
		return scimErrorResponse(c, http.StatusBadRequest, "invalid request body", "invalidValue")
	}

	email := req.UserName
	if email == "" && len(req.Emails) > 0 {
		email = req.Emails[0].Value
	}
	if email == "" {
		return scimErrorResponse(c, http.StatusBadRequest, "userName is required", "invalidValue")
	}
	name := req.Name.Formatted
	if name == "" {
		name = email
	}

	u, err := h.scimUC.ProvisionScimUser(ctx, interfaces.ProvisionScimUserParam{
		Email:       email,
		ExternalID:  req.ExternalID,
		Name:        name,
		Role:        role.RoleReader,
		WorkspaceID: wsID,
	})
	if err != nil {
		return h.mapError(c, err)
	}

	// If the IdP explicitly created the account as inactive, deprovision immediately.
	if req.Active != nil && !*req.Active {
		if err := h.scimUC.DeprovisionScimUserByUserID(ctx, wsID, u.ID()); err != nil {
			return h.mapError(c, err)
		}
	}

	member, err := h.memberForUser(ctx, wsID, u.ID())
	if err != nil {
		return h.mapError(c, err)
	}
	resp := DomainUserToScimUser(u, member, requestBaseURL(c))

	c.Response().Header().Set("Location", resp.Meta.Location)
	return scimJSON(c, http.StatusCreated, resp)
}

// Delete handles DELETE /scim/v2/Users/:id — soft-deprovision (204 No Content per RFC 7644).
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

	if err := h.scimUC.DeprovisionScimUserByUserID(ctx, wsID, uid); err != nil {
		return h.mapError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
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

	member, err := h.memberForUser(ctx, wsID, uid)
	if err != nil {
		return h.mapError(c, err)
	}
	return scimJSON(c, http.StatusOK, DomainUserToScimUser(u, member, requestBaseURL(c)))
}

// List handles GET /scim/v2/Users with optional ?filter=, ?startIndex=, ?count= params.
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

	members, err := h.membersForWorkspace(ctx, wsID)
	if err != nil {
		return h.mapError(c, err)
	}

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
			for _, u := range users {
				if m, ok := members[u.ID()]; ok && m.ExternalID == val {
					filtered = append(filtered, u)
				}
			}
		default:
			return scimErrorResponse(c, http.StatusBadRequest, "unsupported filter attribute", "invalidFilter")
		}
	}

	// SCIM pagination: startIndex is 1-based, count is the max items to return.
	totalResults := len(filtered)
	startIndex := 1
	pageCount := totalResults
	if s := c.QueryParam("startIndex"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v >= 1 {
			startIndex = v
		}
	}
	if cnt := c.QueryParam("count"); cnt != "" {
		if v, err := strconv.Atoi(cnt); err == nil && v >= 0 {
			pageCount = v
		}
	}
	offset := startIndex - 1
	if offset > totalResults {
		offset = totalResults
	}
	end := offset + pageCount
	if end > totalResults {
		end = totalResults
	}
	paged := filtered[offset:end]

	baseURL := requestBaseURL(c)
	resources := make([]ScimUser, 0, len(paged))
	for _, u := range paged {
		member := members[u.ID()]
		resources = append(resources, DomainUserToScimUser(u, member, baseURL))
	}

	return scimJSON(c, http.StatusOK, ScimListResponse{
		ItemsPerPage: len(resources),
		Resources:    resources,
		Schemas:      []string{ScimSchemaListResponse},
		StartIndex:   startIndex,
		TotalResults: totalResults,
	})
}

// Patch handles PATCH /scim/v2/Users/:id — partial update (handles active flag changes).
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

	// Confirm user exists in workspace.
	if _, err := h.scimUC.GetScimUser(ctx, wsID, uid); err != nil {
		return h.mapError(c, err)
	}

	deprovisioned := false
	reactivated := false
	for _, op := range patchOp.Operations {
		if !strings.EqualFold(op.Op, "replace") {
			continue
		}

		// Okta format: {"op":"replace","path":"active","value":false}
		if strings.EqualFold(op.Path, "active") {
			if active, ok := parseBoolValue(op.Value); ok {
				if active {
					reactivated = true
				} else {
					deprovisioned = true
				}
			}
			continue
		}

		// Azure AD format: {"op":"replace","value":{"active":false}}
		if op.Path == "" {
			if active, ok := extractActiveBool(op.Value); ok {
				if active {
					reactivated = true
				} else {
					deprovisioned = true
				}
			}
		}
	}

	if deprovisioned {
		if err := h.scimUC.DeprovisionScimUserByUserID(ctx, wsID, uid); err != nil {
			return h.mapError(c, err)
		}
	} else if reactivated {
		if err := h.scimUC.ReactivateScimUserByUserID(ctx, wsID, uid); err != nil {
			return h.mapError(c, err)
		}
	}

	// Re-fetch user and member to reflect the updated state.
	u, err := h.scimUC.GetScimUser(ctx, wsID, uid)
	if err != nil {
		return h.mapError(c, err)
	}
	member, err := h.memberForUser(ctx, wsID, uid)
	if err != nil {
		return h.mapError(c, err)
	}
	return scimJSON(c, http.StatusOK, DomainUserToScimUser(u, member, requestBaseURL(c)))
}

// Replace handles PUT /scim/v2/Users/:id — full replace.
// Applies the active state from the request body; other attributes are not yet mutable.
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

	var req ScimUserWriteRequest
	if err := c.Bind(&req); err != nil {
		return scimErrorResponse(c, http.StatusBadRequest, "invalid request body", "invalidValue")
	}

	// Apply active state if explicitly specified in the replacement.
	if req.Active != nil {
		if !*req.Active {
			if err := h.scimUC.DeprovisionScimUserByUserID(ctx, wsID, uid); err != nil {
				return h.mapError(c, err)
			}
		} else {
			if err := h.scimUC.ReactivateScimUserByUserID(ctx, wsID, uid); err != nil {
				return h.mapError(c, err)
			}
		}
	}

	u, err := h.scimUC.GetScimUser(ctx, wsID, uid)
	if err != nil {
		return h.mapError(c, err)
	}

	member, err := h.memberForUser(ctx, wsID, uid)
	if err != nil {
		return h.mapError(c, err)
	}
	return scimJSON(c, http.StatusOK, DomainUserToScimUser(u, member, requestBaseURL(c)))
}

// --- helpers ---

// memberForUser retrieves the workspace.Member for a given user within a workspace.
func (h *UserHandler) memberForUser(ctx context.Context, wsID workspace.ID, uid user.ID) (workspace.Member, error) {
	ws, err := h.workspaceRepo.FindByID(ctx, wsID)
	if err != nil {
		return workspace.Member{}, err
	}
	if m := ws.Members().User(uid); m != nil {
		return *m, nil
	}
	return workspace.Member{}, nil
}

// membersForWorkspace returns all members of a workspace as a map.
func (h *UserHandler) membersForWorkspace(ctx context.Context, wsID workspace.ID) (map[workspace.UserID]workspace.Member, error) {
	ws, err := h.workspaceRepo.FindByID(ctx, wsID)
	if err != nil {
		return nil, err
	}
	return ws.Members().Users(), nil
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
	if errors.Is(err, interfaces.ErrOperationDenied) {
		return scimErrorResponse(c, http.StatusBadRequest, "operation denied", "invalidFilter")
	}
	return scimErrorResponse(c, http.StatusInternalServerError, "internal server error", "")
}

// requestBaseURL derives the public-facing base URL from the incoming request,
// respecting X-Forwarded-Proto for reverse-proxy deployments.
func requestBaseURL(c echo.Context) string {
	scheme := "https"
	if proto := c.Request().Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	} else if c.Request().TLS == nil {
		scheme = "http"
	}
	return scheme + "://" + c.Request().Host
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
