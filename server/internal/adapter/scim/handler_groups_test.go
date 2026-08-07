package scim

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	accountmemory "github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interactor"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupGroupHandlerTest creates a fresh in-memory DB, a SCIM-enabled workspace with a group→role mapping,
// and a GroupHandler ready to use.
func setupGroupHandlerTest(t *testing.T) (
	db *repo.Container,
	ws *workspace.Workspace,
	ownerID user.ID,
	handler *GroupHandler,
) {
	t.Helper()
	db = accountmemory.New()

	ownerID = user.NewID()
	owner := user.New().ID(ownerID).Name("Owner").Email("owner@example.com").Workspace(user.NewWorkspaceID()).MustBuild()
	require.NoError(t, db.User.Save(t.Context(), owner))

	cfg := workspace.NewScimConfig()
	cfg.SetEnabled(true)
	cfg.SetGroupRoleMapping(map[string]role.RoleType{
		"Engineering": role.RoleMaintainer,
		"Readers":     role.RoleReader,
	})

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
	handler = NewGroupHandler(scimUC, db.Workspace, testBaseURL)
	return
}

// TestListGroups verifies that List returns one group per occupied role bucket.
func TestListGroups(t *testing.T) {
	db, ws, ownerID, handler := setupGroupHandlerTest(t)

	// Add a maintainer so the "Engineering" bucket is occupied.
	maintainerID := user.NewID()
	maintainer := user.New().ID(maintainerID).Name("Bob").Email("bob@example.com").Workspace(user.NewWorkspaceID()).MustBuild()
	require.NoError(t, db.User.Save(t.Context(), maintainer))

	updatedWS, err := db.Workspace.FindByID(t.Context(), ws.ID())
	require.NoError(t, err)
	require.NoError(t, updatedWS.Members().Join(maintainer, role.RoleMaintainer, ownerID))
	require.NoError(t, db.Workspace.Save(t.Context(), updatedWS))

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Groups", nil)
	req = contextWithWorkspace(req, ws.ID())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, handler.List(c))
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp ScimListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	// Owner (role.RoleOwner) has 1 bucket; maintainer (Engineering) has 1 bucket → 2 total.
	assert.Equal(t, 2, resp.TotalResults)
}

// TestListGroups_Empty verifies that an empty workspace returns an empty group list.
func TestListGroups_Empty(t *testing.T) {
	// Create workspace with no non-disabled members aside from the owner.
	db := accountmemory.New()
	ownerID := user.NewID()
	owner := user.New().ID(ownerID).Name("Owner").Email("owner@example.com").Workspace(user.NewWorkspaceID()).MustBuild()
	require.NoError(t, db.User.Save(t.Context(), owner))

	cfg := workspace.NewScimConfig()
	cfg.SetEnabled(true)
	ws := workspace.New().NewID().Name("e").Alias("e").
		Members(map[user.ID]workspace.Member{}).
		ScimConfig(cfg).MustBuild()
	require.NoError(t, db.Workspace.Save(t.Context(), ws))

	handler := NewGroupHandler(interactor.NewScim(db), db.Workspace, testBaseURL)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Groups", nil)
	req = contextWithWorkspace(req, ws.ID())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, handler.List(c))
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp ScimListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 0, resp.TotalResults)
}

// TestGetGroup verifies that Get returns the correct group by decoded ID.
func TestGetGroup(t *testing.T) {
	_, ws, _, handler := setupGroupHandlerTest(t)

	gid := makeGroupID(ws.ID(), "Engineering")

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Groups/"+gid, nil)
	req = contextWithWorkspace(req, ws.ID())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(gid)

	require.NoError(t, handler.Get(c))
	assert.Equal(t, http.StatusOK, rec.Code)

	var group ScimGroup
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &group))
	assert.Equal(t, "Engineering", group.DisplayName)
	assert.Equal(t, gid, group.ID)
}

// TestGetGroup_InvalidID verifies that an unparseable group ID returns 404.
func TestGetGroup_InvalidID(t *testing.T) {
	_, ws, _, handler := setupGroupHandlerTest(t)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Groups/not-base64!!!", nil)
	req = contextWithWorkspace(req, ws.ID())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("not-base64!!!")

	require.NoError(t, handler.Get(c))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// TestPatchGroup_AddMembers verifies that op:add path:members adds the member and calls SyncScimGroup.
func TestPatchGroup_AddMembers(t *testing.T) {
	db, ws, ownerID, handler := setupGroupHandlerTest(t)

	// Create a new user to add to the Engineering group.
	newUserID := user.NewID()
	newUser := user.New().ID(newUserID).Name("Alice").Email("alice@example.com").Workspace(user.NewWorkspaceID()).MustBuild()
	require.NoError(t, db.User.Save(t.Context(), newUser))

	// Add them to the workspace as a reader first so they're members.
	updatedWS, err := db.Workspace.FindByID(t.Context(), ws.ID())
	require.NoError(t, err)
	require.NoError(t, updatedWS.Members().Join(newUser, role.RoleReader, ownerID))
	require.NoError(t, db.Workspace.Save(t.Context(), updatedWS))

	gid := makeGroupID(ws.ID(), "Engineering")
	body := `{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"add","path":"members","value":[{"value":"` + newUserID.String() + `"}]}]}`

	e := echo.New()
	req := httptest.NewRequest(http.MethodPatch, "/scim/v2/Groups/"+gid, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = contextWithWorkspace(req, ws.ID())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(gid)

	require.NoError(t, handler.Patch(c))
	assert.Equal(t, http.StatusOK, rec.Code)

	// Verify the user is now in the Engineering group (RoleMaintainer).
	finalWS, err := db.Workspace.FindByID(t.Context(), ws.ID())
	require.NoError(t, err)
	assert.Equal(t, role.RoleMaintainer, finalWS.Members().UserRole(newUserID))
}

// TestPatchGroup_RemoveMembers verifies that op:remove path:members soft-disables the member.
func TestPatchGroup_RemoveMembers(t *testing.T) {
	db, ws, ownerID, handler := setupGroupHandlerTest(t)

	// Provision a reader-role member.
	readerID := user.NewID()
	reader := user.New().ID(readerID).Name("Reader").Email("reader@example.com").Workspace(user.NewWorkspaceID()).MustBuild()
	require.NoError(t, db.User.Save(t.Context(), reader))
	updatedWS, err := db.Workspace.FindByID(t.Context(), ws.ID())
	require.NoError(t, err)
	require.NoError(t, updatedWS.Members().Join(reader, role.RoleReader, ownerID))
	require.NoError(t, updatedWS.Members().SetUserExternalID(readerID, "ext-reader"))
	require.NoError(t, db.Workspace.Save(t.Context(), updatedWS))

	gid := makeGroupID(ws.ID(), "Readers")
	body := `{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"remove","path":"members","value":[{"value":"` + readerID.String() + `"}]}]}`

	e := echo.New()
	req := httptest.NewRequest(http.MethodPatch, "/scim/v2/Groups/"+gid, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = contextWithWorkspace(req, ws.ID())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(gid)

	require.NoError(t, handler.Patch(c))
	assert.Equal(t, http.StatusOK, rec.Code)

	// The member should now be disabled.
	finalWS, err := db.Workspace.FindByID(t.Context(), ws.ID())
	require.NoError(t, err)
	m := finalWS.Members().User(readerID)
	require.NotNil(t, m)
	assert.True(t, m.Disabled)
}

// TestPatchGroup_RemoveLastOwner verifies that the last-owner guard prevents the last owner
// from being disabled even when they are excluded from the incoming member list.
// SyncScimGroup silently preserves the owner (returns 200) rather than rejecting with 409.
func TestPatchGroup_RemoveLastOwner(t *testing.T) {
	db, ws, ownerID, handler := setupGroupHandlerTest(t)

	// Give the owner an ExternalID so the soft-disable path runs.
	updatedWS, err := db.Workspace.FindByID(t.Context(), ws.ID())
	require.NoError(t, err)
	require.NoError(t, updatedWS.Members().SetUserExternalID(ownerID, "ext-owner"))
	require.NoError(t, db.Workspace.Save(t.Context(), updatedWS))

	// The owner is in the "owner" role bucket (no mapping — falls back to role name).
	ownerGroupName := string(role.RoleOwner)
	gid := makeGroupID(ws.ID(), ownerGroupName)
	// Send an empty member list to try to remove all members.
	body := `{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"remove","path":"members","value":[{"value":"` + ownerID.String() + `"}]}]}`

	e := echo.New()
	req := httptest.NewRequest(http.MethodPatch, "/scim/v2/Groups/"+gid, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = contextWithWorkspace(req, ws.ID())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(gid)

	require.NoError(t, handler.Patch(c))
	// SyncScimGroup silently preserves the last owner (does not return 409).
	assert.Equal(t, http.StatusOK, rec.Code)

	// Verify the owner is still active.
	finalWS, err := db.Workspace.FindByID(t.Context(), ws.ID())
	require.NoError(t, err)
	m := finalWS.Members().User(ownerID)
	require.NotNil(t, m)
	assert.False(t, m.Disabled, "last owner must NOT be disabled by SyncScimGroup")
}

// TestDeleteGroup verifies that Delete removes the group mapping without deprovisioning members.
func TestDeleteGroup(t *testing.T) {
	db, ws, ownerID, handler := setupGroupHandlerTest(t)

	// Add a maintainer member that should NOT be deprovisioned after delete.
	maintainerID := user.NewID()
	maintainer := user.New().ID(maintainerID).Name("Maintainer").Email("maintainer@example.com").Workspace(user.NewWorkspaceID()).MustBuild()
	require.NoError(t, db.User.Save(t.Context(), maintainer))
	updatedWS, err := db.Workspace.FindByID(t.Context(), ws.ID())
	require.NoError(t, err)
	require.NoError(t, updatedWS.Members().Join(maintainer, role.RoleMaintainer, ownerID))
	require.NoError(t, db.Workspace.Save(t.Context(), updatedWS))

	gid := makeGroupID(ws.ID(), "Engineering")

	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/scim/v2/Groups/"+gid, nil)
	req = contextWithWorkspace(req, ws.ID())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(gid)

	require.NoError(t, handler.Delete(c))
	assert.Equal(t, http.StatusNoContent, rec.Code)

	// Mapping should be removed.
	finalWS, err := db.Workspace.FindByID(t.Context(), ws.ID())
	require.NoError(t, err)
	mapping := finalWS.ScimConfig().GroupRoleMapping()
	_, stillInMapping := mapping["Engineering"]
	assert.False(t, stillInMapping, "Engineering group should have been removed from mapping")

	// Members should NOT have been deprovisioned.
	m := finalWS.Members().User(maintainerID)
	require.NotNil(t, m)
	assert.False(t, m.Disabled, "member should NOT be deprovisioned after group delete")
}

// TestReplaceGroup verifies that Replace does a full member sync.
func TestReplaceGroup(t *testing.T) {
	db, ws, ownerID, handler := setupGroupHandlerTest(t)

	// Provision one existing maintainer and one new user to replace with.
	existingID := user.NewID()
	existing := user.New().ID(existingID).Name("Existing").Email("existing@example.com").Workspace(user.NewWorkspaceID()).MustBuild()
	require.NoError(t, db.User.Save(t.Context(), existing))

	newUserID := user.NewID()
	newUser := user.New().ID(newUserID).Name("NewUser").Email("newuser@example.com").Workspace(user.NewWorkspaceID()).MustBuild()
	require.NoError(t, db.User.Save(t.Context(), newUser))

	updatedWS, err := db.Workspace.FindByID(t.Context(), ws.ID())
	require.NoError(t, err)
	require.NoError(t, updatedWS.Members().Join(existing, role.RoleMaintainer, ownerID))
	require.NoError(t, updatedWS.Members().SetUserExternalID(existingID, "ext-existing"))
	require.NoError(t, updatedWS.Members().Join(newUser, role.RoleReader, ownerID))
	require.NoError(t, db.Workspace.Save(t.Context(), updatedWS))

	gid := makeGroupID(ws.ID(), "Engineering")
	// Replace Engineering group with only newUser.
	body := `{"displayName":"Engineering","schemas":["` + ScimSchemaGroup + `"],"members":[{"value":"` + newUserID.String() + `"}]}`

	e := echo.New()
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Groups/"+gid, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = contextWithWorkspace(req, ws.ID())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(gid)

	require.NoError(t, handler.Replace(c))
	assert.Equal(t, http.StatusOK, rec.Code)

	// existingID (had ExternalID) should now be disabled; newUserID should be maintainer.
	finalWS, err := db.Workspace.FindByID(t.Context(), ws.ID())
	require.NoError(t, err)
	existingMember := finalWS.Members().User(existingID)
	require.NotNil(t, existingMember)
	assert.True(t, existingMember.Disabled, "old member should be disabled after full replace")
	assert.Equal(t, role.RoleMaintainer, finalWS.Members().UserRole(newUserID))
}
