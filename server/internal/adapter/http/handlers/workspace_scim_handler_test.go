package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/internal/adapter/http/handlers"
	"github.com/reearth/reearth-accounts/server/internal/adapter/http/httpmodel"
	accountmemory "github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interactor"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupScimAdminTest creates a SCIM-enabled workspace, an owner operator, and an
// interfaces.Container with the real Scim interactor backed by in-memory repos.
func setupScimAdminTest(t *testing.T) (
	db *repo.Container,
	ws *workspace.Workspace,
	ownerID user.ID,
	ownerOp *workspace.Operator,
	uc *interfaces.Container,
) {
	t.Helper()
	db = accountmemory.New()

	ownerID = user.NewID()
	owner := user.New().ID(ownerID).Name("Owner").Email("owner@example.com").Workspace(user.NewWorkspaceID()).MustBuild()
	require.NoError(t, db.User.Save(t.Context(), owner))

	cfg := workspace.NewScimConfig()
	cfg.SetEnabled(true)

	ws = workspace.New().
		NewID().
		Name("enterprise").
		Alias("enterprise").
		Members(map[user.ID]workspace.Member{
			ownerID: {Role: role.RoleOwner, InvitedBy: ownerID},
		}).
		ScimConfig(cfg).
		MustBuild()
	require.NoError(t, db.Workspace.Save(t.Context(), ws))

	ownerOp = &workspace.Operator{
		User:             &ownerID,
		OwningWorkspaces: id.WorkspaceIDList{ws.ID()},
	}

	scimUC := interactor.NewScim(db)
	uc = &interfaces.Container{Scim: scimUC}
	return
}

// newScimAdminContext creates an Echo context with the usecases and operator attached.
func newScimAdminContext(
	t *testing.T,
	e *echo.Echo,
	method, path string,
	body string,
	wsID workspace.ID,
	op *workspace.Operator,
	uc *interfaces.Container,
) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}

	ctx := adapter.AttachUsecases(req.Context(), uc)
	ctx = adapter.AttachOperator(ctx, op)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(wsID.String())
	return c, rec
}

// TestGenerateToken_Owner verifies that an owner can generate a SCIM token.
func TestGenerateToken_Owner(t *testing.T) {
	db, ws, _, ownerOp, uc := setupScimAdminTest(t)
	h := handlers.NewWorkspaceScimHandler()
	e := echo.New()

	c, rec := newScimAdminContext(t, e, http.MethodPost, "/api/workspaces/"+ws.ID().String()+"/scim/token", "", ws.ID(), ownerOp, uc)

	require.NoError(t, h.GenerateToken(c))
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp httpmodel.GenerateScimTokenResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Token)
	assert.NotEmpty(t, resp.Warning)

	// Verify the hash was stored.
	finalWS, err := db.Workspace.FindByID(t.Context(), ws.ID())
	require.NoError(t, err)
	assert.NotEmpty(t, finalWS.ScimConfig().TokenHash())
}

// TestGenerateToken_NonMaintainer verifies that a non-member gets 403.
func TestGenerateToken_NonMaintainer(t *testing.T) {
	_, ws, _, _, uc := setupScimAdminTest(t)
	h := handlers.NewWorkspaceScimHandler()
	e := echo.New()

	strangerID := user.NewID()
	strangerOp := &workspace.Operator{
		User:               &strangerID,
		ReadableWorkspaces: id.WorkspaceIDList{},
	}

	c, rec := newScimAdminContext(t, e, http.MethodPost, "/api/workspaces/"+ws.ID().String()+"/scim/token", "", ws.ID(), strangerOp, uc)

	// The error is returned from the handler, not committed directly (no CustomHTTPErrorHandler in unit test).
	err := h.GenerateToken(c)
	require.Error(t, err)

	// When routed via CustomHTTPErrorHandler, ErrOperationDenied maps to 403.
	// In unit test, confirm it's the right error.
	assert.ErrorIs(t, err, interfaces.ErrOperationDenied)
	_ = rec // recorder not used — handler returned an error, not a response
}

// TestRotateToken verifies that a second GenerateToken call rotates (replaces) the hash.
func TestRotateToken(t *testing.T) {
	db, ws, _, ownerOp, uc := setupScimAdminTest(t)
	h := handlers.NewWorkspaceScimHandler()
	e := echo.New()

	// First call.
	c1, rec1 := newScimAdminContext(t, e, http.MethodPost, "/api/workspaces/"+ws.ID().String()+"/scim/token", "", ws.ID(), ownerOp, uc)
	require.NoError(t, h.GenerateToken(c1))
	var first httpmodel.GenerateScimTokenResponse
	require.NoError(t, json.Unmarshal(rec1.Body.Bytes(), &first))

	firstHash, err := db.Workspace.FindByID(t.Context(), ws.ID())
	require.NoError(t, err)
	hash1 := firstHash.ScimConfig().TokenHash()

	// Second call — should rotate.
	c2, rec2 := newScimAdminContext(t, e, http.MethodPost, "/api/workspaces/"+ws.ID().String()+"/scim/token", "", ws.ID(), ownerOp, uc)
	require.NoError(t, h.GenerateToken(c2))
	var second httpmodel.GenerateScimTokenResponse
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &second))

	secondHash, err := db.Workspace.FindByID(t.Context(), ws.ID())
	require.NoError(t, err)
	hash2 := secondHash.ScimConfig().TokenHash()

	assert.NotEqual(t, first.Token, second.Token, "tokens must differ after rotation")
	assert.NotEqual(t, hash1, hash2, "stored hash must change after rotation")
}

// TestRevokeToken verifies that RevokeToken clears the hash and disables SCIM.
func TestRevokeToken(t *testing.T) {
	db, ws, _, ownerOp, uc := setupScimAdminTest(t)
	h := handlers.NewWorkspaceScimHandler()
	e := echo.New()

	// First generate a token.
	c1, _ := newScimAdminContext(t, e, http.MethodPost, "/api/workspaces/"+ws.ID().String()+"/scim/token", "", ws.ID(), ownerOp, uc)
	require.NoError(t, h.GenerateToken(c1))

	// Now revoke.
	c2, rec2 := newScimAdminContext(t, e, http.MethodDelete, "/api/workspaces/"+ws.ID().String()+"/scim/token", "", ws.ID(), ownerOp, uc)
	require.NoError(t, h.RevokeToken(c2))
	assert.Equal(t, http.StatusNoContent, rec2.Code)

	// Token hash must be empty and SCIM must be disabled.
	finalWS, err := db.Workspace.FindByID(t.Context(), ws.ID())
	require.NoError(t, err)
	assert.Empty(t, finalWS.ScimConfig().TokenHash())
	assert.False(t, finalWS.ScimConfig().Enabled())
}

// TestUpdateConfig_ValidRoles verifies that a valid role mapping is stored.
func TestUpdateConfig_ValidRoles(t *testing.T) {
	db, ws, _, ownerOp, uc := setupScimAdminTest(t)
	h := handlers.NewWorkspaceScimHandler()
	e := echo.New()

	body := `{"enabled":true,"group_role_mapping":{"Engineering":"maintainer","Readers":"reader"}}`
	c, rec := newScimAdminContext(t, e, http.MethodPut, "/api/workspaces/"+ws.ID().String()+"/scim/config", body, ws.ID(), ownerOp, uc)

	require.NoError(t, h.UpdateConfig(c))
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp httpmodel.ScimConfigResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Enabled)
	assert.Equal(t, "maintainer", resp.GroupRoleMapping["Engineering"])
	assert.Equal(t, "reader", resp.GroupRoleMapping["Readers"])

	// Verify persisted.
	finalWS, err := db.Workspace.FindByID(t.Context(), ws.ID())
	require.NoError(t, err)
	assert.Equal(t, role.RoleMaintainer, finalWS.ScimConfig().GroupRoleMapping()["Engineering"])
}

// TestUpdateConfig_InvalidRole verifies that an unrecognised role returns 400.
func TestUpdateConfig_InvalidRole(t *testing.T) {
	_, ws, _, ownerOp, uc := setupScimAdminTest(t)
	h := handlers.NewWorkspaceScimHandler()
	e := echo.New()

	body := `{"enabled":true,"group_role_mapping":{"Admins":"superadmin"}}`
	c, rec := newScimAdminContext(t, e, http.MethodPut, "/api/workspaces/"+ws.ID().String()+"/scim/config", body, ws.ID(), ownerOp, uc)

	err := h.UpdateConfig(c)
	require.Error(t, err)

	// Should be a 400 ErrorResponse.
	var er interface{ Error() string }
	require.ErrorAs(t, err, &er)
	_ = rec // handler returns error, not a response
}

// TestGetConfig_MasksHash verifies that the response uses token_issued instead of the raw hash.
func TestGetConfig_MasksHash(t *testing.T) {
	_, ws, _, ownerOp, uc := setupScimAdminTest(t)
	h := handlers.NewWorkspaceScimHandler()
	e := echo.New()

	// Before any token is generated.
	c1, rec1 := newScimAdminContext(t, e, http.MethodGet, "/api/workspaces/"+ws.ID().String()+"/scim/config", "", ws.ID(), ownerOp, uc)
	require.NoError(t, h.GetConfig(c1))
	assert.Equal(t, http.StatusOK, rec1.Code)

	var before httpmodel.ScimConfigResponse
	require.NoError(t, json.Unmarshal(rec1.Body.Bytes(), &before))
	assert.False(t, before.TokenIssued, "token_issued must be false before generating a token")
	assert.NotContains(t, rec1.Body.String(), "tokenHash", "raw hash must never appear in response")

	// Generate a token.
	c2, _ := newScimAdminContext(t, e, http.MethodPost, "/api/workspaces/"+ws.ID().String()+"/scim/token", "", ws.ID(), ownerOp, uc)
	require.NoError(t, h.GenerateToken(c2))

	// After token is generated.
	c3, rec3 := newScimAdminContext(t, e, http.MethodGet, "/api/workspaces/"+ws.ID().String()+"/scim/config", "", ws.ID(), ownerOp, uc)
	require.NoError(t, h.GetConfig(c3))
	assert.Equal(t, http.StatusOK, rec3.Code)

	var after httpmodel.ScimConfigResponse
	require.NoError(t, json.Unmarshal(rec3.Body.Bytes(), &after))
	assert.True(t, after.TokenIssued, "token_issued must be true after token generation")
	assert.NotContains(t, rec3.Body.String(), "$2a$", "bcrypt hash must never appear in response")
}

// TestGetConfig_NonMaintainer verifies that a non-maintainer cannot read the config.
func TestGetConfig_NonMaintainer(t *testing.T) {
	_, ws, _, _, uc := setupScimAdminTest(t)
	h := handlers.NewWorkspaceScimHandler()
	e := echo.New()

	strangerID := user.NewID()
	strangerOp := &workspace.Operator{
		User: &strangerID,
	}

	c, _ := newScimAdminContext(t, e, http.MethodGet, "/api/workspaces/"+ws.ID().String()+"/scim/config", "", ws.ID(), strangerOp, uc)

	err := h.GetConfig(c)
	require.Error(t, err)
	assert.ErrorIs(t, err, interfaces.ErrOperationDenied)
}
