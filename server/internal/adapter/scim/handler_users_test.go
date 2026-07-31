package scim

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	accountmemory "github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interactor"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testBaseURL = "https://accounts.example.com"

// setupHandlerTest creates a fresh in-memory DB, a SCIM-enabled workspace, and a UserHandler.
func setupHandlerTest(t *testing.T) (
	db *repo.Container,
	ws *workspace.Workspace,
	ownerID user.ID,
	handler *UserHandler,
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

	scimUC := interactor.NewScim(db)
	handler = NewUserHandler(scimUC, db.Workspace, testBaseURL)
	return
}

// provisionTestUser is a helper to provision a standard test user.
func provisionTestUser(t *testing.T, ctx context.Context, db *repo.Container, wsID workspace.ID) *user.User {
	t.Helper()
	scimUC := interactor.NewScim(db)
	u, err := scimUC.ProvisionScimUser(ctx, interfaces.ProvisionScimUserParam{
		Email:       "alice@example.com",
		ExternalID:  "ext-alice-001",
		Name:        "Alice",
		Role:        role.RoleWriter,
		WorkspaceID: wsID,
	})
	require.NoError(t, err)
	return u
}

// contextWithWorkspace injects a workspace ID into the request context (simulating middleware).
func contextWithWorkspace(req *http.Request, wsID workspace.ID) *http.Request {
	ctx := context.WithValue(req.Context(), scimWorkspaceKey{}, wsID)
	return req.WithContext(ctx)
}

// --- List ---

func TestUserHandler_List_OnlyOwner(t *testing.T) {
	// The owner is a non-disabled member, so they appear in the list.
	_, ws, _, handler := setupHandlerTest(t)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users", nil)
	req = contextWithWorkspace(req, ws.ID())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.List(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp ScimListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	// The owner is a member, so there is 1 result.
	assert.Equal(t, 1, resp.TotalResults)
}

func TestUserHandler_List_WithUsers(t *testing.T) {
	// After provisioning Alice, the list has the owner + Alice = 2 members.
	db, ws, _, handler := setupHandlerTest(t)

	provisionTestUser(t, t.Context(), db, ws.ID())

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users", nil)
	req = contextWithWorkspace(req, ws.ID())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.List(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp ScimListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 2, resp.TotalResults)
}

func TestUserHandler_List_FilterByUserName(t *testing.T) {
	db, ws, _, handler := setupHandlerTest(t)
	provisionTestUser(t, t.Context(), db, ws.ID())

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users", nil)
	req = contextWithWorkspace(req, ws.ID())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.QueryParams().Set("filter", `userName eq "alice@example.com"`)

	err := handler.List(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp ScimListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp.TotalResults)
}

func TestUserHandler_List_FilterByUserName_NoMatch(t *testing.T) {
	db, ws, _, handler := setupHandlerTest(t)
	provisionTestUser(t, t.Context(), db, ws.ID())

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users", nil)
	req = contextWithWorkspace(req, ws.ID())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.QueryParams().Set("filter", `userName eq "nobody@example.com"`)

	err := handler.List(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp ScimListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 0, resp.TotalResults)
}

func TestUserHandler_List_FilterByExternalID(t *testing.T) {
	db, ws, _, handler := setupHandlerTest(t)
	provisionTestUser(t, t.Context(), db, ws.ID())

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users", nil)
	req = contextWithWorkspace(req, ws.ID())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.QueryParams().Set("filter", `externalId eq "ext-alice-001"`)

	err := handler.List(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp ScimListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp.TotalResults)
}

func TestUserHandler_List_FilterInvalidAttr(t *testing.T) {
	_, ws, _, handler := setupHandlerTest(t)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users", nil)
	req = contextWithWorkspace(req, ws.ID())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.QueryParams().Set("filter", `displayName eq "Alice"`)

	err := handler.List(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Create ---

func TestUserHandler_Create(t *testing.T) {
	_, ws, _, handler := setupHandlerTest(t)

	body := `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"newuser@example.com","externalId":"ext-new-001","name":{"formatted":"New User"}}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req = contextWithWorkspace(req, ws.ID())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Create(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Location"))

	var resp ScimUser
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "newuser@example.com", resp.UserName)
	assert.True(t, resp.Active)
	assert.Equal(t, "ext-new-001", resp.ExternalID)
}

func TestUserHandler_Create_LocationHeader(t *testing.T) {
	_, ws, _, handler := setupHandlerTest(t)

	body := `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"user2@example.com","externalId":"ext-002","name":{"formatted":"User Two"}}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req = contextWithWorkspace(req, ws.ID())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Create(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)

	location := rec.Header().Get("Location")
	assert.True(t, strings.HasPrefix(location, testBaseURL+"/scim/v2/Users/"), "location must point to the created user")
}

// --- Get ---

func TestUserHandler_Get_NotFound(t *testing.T) {
	_, ws, _, handler := setupHandlerTest(t)

	unknownID := user.NewID()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users/"+unknownID.String(), nil)
	req = contextWithWorkspace(req, ws.ID())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(unknownID.String())

	err := handler.Get(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUserHandler_Get_Found(t *testing.T) {
	db, ws, _, handler := setupHandlerTest(t)
	u := provisionTestUser(t, t.Context(), db, ws.ID())

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users/"+u.ID().String(), nil)
	req = contextWithWorkspace(req, ws.ID())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(u.ID().String())

	err := handler.Get(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp ScimUser
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, u.ID().String(), resp.ID)
	assert.Equal(t, "alice@example.com", resp.UserName)
	assert.True(t, resp.Active)
}

// --- Delete ---

func TestUserHandler_Delete(t *testing.T) {
	db, ws, _, handler := setupHandlerTest(t)
	u := provisionTestUser(t, t.Context(), db, ws.ID())

	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/scim/v2/Users/"+u.ID().String(), nil)
	req = contextWithWorkspace(req, ws.ID())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(u.ID().String())

	err := handler.Delete(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp ScimUser
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Active, "deprovisioned user must have active:false")

	// Verify soft-disabled in workspace.
	saved, err := db.Workspace.FindByID(t.Context(), ws.ID())
	require.NoError(t, err)
	m := saved.Members().User(u.ID())
	require.NotNil(t, m)
	assert.True(t, m.Disabled)
}

func TestUserHandler_Delete_LastOwner_Conflict(t *testing.T) {
	db, ws, ownerID, handler := setupHandlerTest(t)

	// Set an ExternalID for the owner.
	ws2, err := db.Workspace.FindByID(t.Context(), ws.ID())
	require.NoError(t, err)
	require.NoError(t, ws2.Members().SetUserExternalID(ownerID, "ext-owner-001"))
	require.NoError(t, db.Workspace.Save(t.Context(), ws2))

	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/scim/v2/Users/"+ownerID.String(), nil)
	req = contextWithWorkspace(req, ws.ID())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(ownerID.String())

	err = handler.Delete(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, rec.Code)
}

// --- Patch ---

func TestUserHandler_Patch_OktaDeprovisioning(t *testing.T) {
	db, ws, _, handler := setupHandlerTest(t)
	u := provisionTestUser(t, t.Context(), db, ws.ID())

	// Okta format: path="active", value=false
	body := `{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"replace","path":"active","value":false}]}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPatch, "/scim/v2/Users/"+u.ID().String(), strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req = contextWithWorkspace(req, ws.ID())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(u.ID().String())

	err := handler.Patch(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	saved, err := db.Workspace.FindByID(t.Context(), ws.ID())
	require.NoError(t, err)
	m := saved.Members().User(u.ID())
	require.NotNil(t, m)
	assert.True(t, m.Disabled)
}

func TestUserHandler_Patch_AzureADDeprovisioning(t *testing.T) {
	db, ws, _, handler := setupHandlerTest(t)
	u := provisionTestUser(t, t.Context(), db, ws.ID())

	// Azure AD format: no path, value is object with active:false
	body := `{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"replace","value":{"active":false}}]}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPatch, "/scim/v2/Users/"+u.ID().String(), strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req = contextWithWorkspace(req, ws.ID())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(u.ID().String())

	err := handler.Patch(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	saved, err := db.Workspace.FindByID(t.Context(), ws.ID())
	require.NoError(t, err)
	m := saved.Members().User(u.ID())
	require.NotNil(t, m)
	assert.True(t, m.Disabled)
}

func TestUserHandler_Patch_NoActiveChange(t *testing.T) {
	db, ws, _, handler := setupHandlerTest(t)
	u := provisionTestUser(t, t.Context(), db, ws.ID())

	// PATCH with unrelated operation - user should remain active.
	body := `{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"replace","path":"active","value":true}]}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPatch, "/scim/v2/Users/"+u.ID().String(), strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req = contextWithWorkspace(req, ws.ID())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(u.ID().String())

	err := handler.Patch(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	saved, err := db.Workspace.FindByID(t.Context(), ws.ID())
	require.NoError(t, err)
	m := saved.Members().User(u.ID())
	require.NotNil(t, m)
	assert.False(t, m.Disabled, "user should remain active")
}

// --- parseFilter helper ---

func TestParseFilter(t *testing.T) {
	tests := []struct {
		input   string
		attr    string
		op      string
		val     string
		wantErr bool
	}{
		{`userName eq "alice@example.com"`, "username", "eq", "alice@example.com", false},
		{`externalId eq "ext-001"`, "externalid", "eq", "ext-001", false},
		{`badfilter`, "", "", "", true},
		{`attr ne "x"`, "attr", "ne", "x", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			attr, op, val, err := parseFilter(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.attr, attr)
				assert.Equal(t, tt.op, op)
				assert.Equal(t, tt.val, val)
			}
		})
	}
}

