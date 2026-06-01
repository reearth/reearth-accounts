// Package conformance holds a single, parameterized repository test suite that
// every persistence backend (memory, mongo, postgres) must satisfy. Backends
// declare which optional behaviors they support via Caps so the shared suite can
// gate backend-specific assertions instead of forking per backend.
//
// Running the same assertions against mongo and postgres is the consistency
// proof: where the two backends are expected to behave identically, the test is
// ungated and must pass on both.
package conformance

import (
	"context"
	"testing"
	"time"

	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/config"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/rerror"
	"github.com/reearth/reearthx/usecasex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Caps describes optional behaviors a backend may or may not implement.
// mongo and postgres set all of these true; memory (an in-process map) sets them
// false because it does not enforce them.
type Caps struct {
	RealTransactions     bool // Begin/Commit/Rollback actually isolate writes
	EnforcesFilter       bool // Filtered() enforces read/write permissions
	OrderedFindByIDs     bool // user FindByIDs preserves request order with nil holes
	RealPagination       bool // cursor/offset pagination is honored
	UniqueEmail          bool // duplicate emails are rejected on Create
	SubstringSearch      bool // FindByNameOrAlias matches case-insensitive substrings
	CaseInsensitiveEmail bool // FindByEmail/FindByAlias match case-insensitively
}

// Factory returns a fresh, empty repo.Container, its capabilities, and cleanup.
type Factory func(t *testing.T) (*repo.Container, Caps, func())

// Run executes the full conformance suite against the given backend factory.
func Run(t *testing.T, nc Factory) {
	// user
	t.Run("User_CRUD", func(t *testing.T) { testUserCRUD(t, nc) })
	t.Run("User_SaveUpdate", func(t *testing.T) { testUserSaveUpdate(t, nc) })
	t.Run("User_FindAll", func(t *testing.T) { testUserFindAll(t, nc) })
	t.Run("User_FindByName_Alias_NameOrEmail", func(t *testing.T) { testUserFindByNameAliasEmail(t, nc) })
	t.Run("User_FindByNameOrAlias", func(t *testing.T) { testUserFindByNameOrAlias(t, nc) })
	t.Run("User_FindByNameOrAlias_Substring", func(t *testing.T) { testUserFindByNameOrAliasSubstring(t, nc) })
	t.Run("User_CaseInsensitiveEmailAlias", func(t *testing.T) { testUserCaseInsensitiveEmailAlias(t, nc) })
	t.Run("User_FindBySub", func(t *testing.T) { testUserFindBySub(t, nc) })
	t.Run("User_FindBySubOrCreate", func(t *testing.T) { testUserFindBySubOrCreate(t, nc) })
	t.Run("User_FindByVerification_PasswordReset", func(t *testing.T) { testUserFindByVerification(t, nc) })
	t.Run("User_DuplicateEmail", func(t *testing.T) { testUserDuplicateEmail(t, nc) })
	t.Run("User_FindByIDs_Ordering", func(t *testing.T) { testUserFindByIDsOrdering(t, nc) })
	t.Run("User_SearchByKeyword", func(t *testing.T) { testUserSearch(t, nc) })
	t.Run("User_Pagination", func(t *testing.T) { testUserPagination(t, nc) })
	// workspace
	t.Run("Workspace_CRUD_Members", func(t *testing.T) { testWorkspaceCRUD(t, nc) })
	t.Run("Workspace_SaveUpdate", func(t *testing.T) { testWorkspaceSaveUpdate(t, nc) })
	t.Run("Workspace_FindByName_Alias", func(t *testing.T) { testWorkspaceFindByNameAlias(t, nc) })
	t.Run("Workspace_FindByAliases", func(t *testing.T) { testWorkspaceFindByAliases(t, nc) })
	t.Run("Workspace_FindByIDs", func(t *testing.T) { testWorkspaceFindByIDs(t, nc) })
	t.Run("Workspace_FindByUser", func(t *testing.T) { testWorkspaceFindByUser(t, nc) })
	t.Run("Workspace_FindByIntegration", func(t *testing.T) { testWorkspaceFindByIntegration(t, nc) })
	t.Run("Workspace_SaveAll_RemoveAll", func(t *testing.T) { testWorkspaceSaveAllRemoveAll(t, nc) })
	t.Run("Workspace_Remove", func(t *testing.T) { testWorkspaceRemove(t, nc) })
	t.Run("Workspace_Filtered", func(t *testing.T) { testWorkspaceFiltered(t, nc) })
	// role
	t.Run("Role_CRUD", func(t *testing.T) { testRoleCRUD(t, nc) })
	t.Run("Role_FindAll_FindByIDs", func(t *testing.T) { testRoleFindAllAndByIDs(t, nc) })
	// permittable
	t.Run("Permittable_RoleQueries", func(t *testing.T) { testPermittable(t, nc) })
	t.Run("Permittable_WorkspaceRoles", func(t *testing.T) { testPermittableWorkspaceRoles(t, nc) })
	t.Run("Permittable_FindByUserIDs_SaveMany", func(t *testing.T) { testPermittableFindByUserIDsAndSaveMany(t, nc) })
	t.Run("Permittable_NotFound", func(t *testing.T) { testPermittableNotFound(t, nc) })
	// config
	t.Run("Config_LockLoadSave", func(t *testing.T) { testConfig(t, nc) })
	t.Run("Config_SaveAuth", func(t *testing.T) { testConfigSaveAuth(t, nc) })
	// transaction
	t.Run("Transaction_CommitRollback", func(t *testing.T) { testTransaction(t, nc) })
	t.Run("Transactor_Wiring", func(t *testing.T) { testTransactorWiring(t, nc) })
}

func newUser(t *testing.T, name, email string) *user.User {
	t.Helper()
	u, err := user.New().NewID().Name(name).Email(email).Workspace(id.NewWorkspaceID()).Build()
	require.NoError(t, err)
	return u
}

// timeFixed is a stable timestamp (UTC, second precision) for fields that must
// round-trip identically across backends.
func timeFixed() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }

func newWorkspace(t *testing.T, name string, owner id.UserID) *workspace.Workspace {
	t.Helper()
	ws, err := workspace.New().NewID().Name(name).
		Members(map[id.UserID]workspace.Member{owner: {Role: role.RoleOwner, InvitedBy: owner}}).Build()
	require.NoError(t, err)
	return ws
}

// ---- user ----

func testUserCRUD(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	uid := id.NewUserID()
	wid := id.NewWorkspaceID()
	u, err := user.New().ID(uid).Name("alice").Email("alice@example.com").Workspace(wid).Alias("alice").Build()
	require.NoError(t, err)

	require.NoError(t, c.User.Create(ctx, u))
	got, err := c.User.FindByID(ctx, uid)
	require.NoError(t, err)
	assert.Equal(t, "alice@example.com", got.Email())

	byEmail, err := c.User.FindByEmail(ctx, "alice@example.com")
	require.NoError(t, err)
	assert.Equal(t, uid, byEmail.ID())

	// not found
	_, err = c.User.FindByEmail(ctx, "nobody@example.com")
	assert.ErrorIs(t, err, rerror.ErrNotFound)

	require.NoError(t, c.User.Remove(ctx, uid))
	_, err = c.User.FindByID(ctx, uid)
	assert.ErrorIs(t, err, rerror.ErrNotFound)
}

func testUserSaveUpdate(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	u := newUser(t, "save-me", "save@example.com")
	require.NoError(t, c.User.Save(ctx, u))

	u.UpdateName("renamed")
	require.NoError(t, c.User.Save(ctx, u))

	got, err := c.User.FindByID(ctx, u.ID())
	require.NoError(t, err)
	assert.Equal(t, "renamed", got.Name())
}

func testUserFindAll(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	a := newUser(t, "all-a", "alla@example.com")
	b := newUser(t, "all-b", "allb@example.com")
	require.NoError(t, c.User.Create(ctx, a))
	require.NoError(t, c.User.Create(ctx, b))
	all, err := c.User.FindAll(ctx)
	require.NoError(t, err)
	ids := map[id.UserID]bool{}
	for _, u := range all {
		ids[u.ID()] = true
	}
	assert.True(t, ids[a.ID()] && ids[b.ID()])
}

func testUserFindByNameAliasEmail(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	u, err := user.New().NewID().Name("Findable").Alias("findalias").Email("find@example.com").
		Workspace(id.NewWorkspaceID()).Build()
	require.NoError(t, err)
	require.NoError(t, c.User.Create(ctx, u))

	byName, err := c.User.FindByName(ctx, "Findable")
	require.NoError(t, err)
	assert.Equal(t, u.ID(), byName.ID())

	byAlias, err := c.User.FindByAlias(ctx, "findalias")
	require.NoError(t, err)
	assert.Equal(t, u.ID(), byAlias.ID())

	byNOE, err := c.User.FindByNameOrEmail(ctx, "find@example.com")
	require.NoError(t, err)
	assert.Equal(t, u.ID(), byNOE.ID())
	byNOE2, err := c.User.FindByNameOrEmail(ctx, "Findable")
	require.NoError(t, err)
	assert.Equal(t, u.ID(), byNOE2.ID())
}

func testUserFindByNameOrAlias(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	u, err := user.New().NewID().Name("Zaphod").Alias("zalias").Email("z@example.com").
		Workspace(id.NewWorkspaceID()).Build()
	require.NoError(t, err)
	require.NoError(t, c.User.Create(ctx, u))

	// full-term match works on every backend
	res, err := c.User.FindByNameOrAlias(ctx, "Zaphod")
	require.NoError(t, err)
	require.NotEmpty(t, res)
	assert.Equal(t, u.ID(), res[0].ID())
}

func testUserFindByNameOrAliasSubstring(t *testing.T, nc Factory) {
	c, caps, done := nc(t)
	defer done()
	if !caps.SubstringSearch {
		t.Skip("backend does not implement case-insensitive substring FindByNameOrAlias")
	}
	ctx := context.Background()
	u, err := user.New().NewID().Name("Beeblebrox").Alias("president").Email("bb@example.com").
		Workspace(id.NewWorkspaceID()).Build()
	require.NoError(t, err)
	require.NoError(t, c.User.Create(ctx, u))

	res, err := c.User.FindByNameOrAlias(ctx, "EBLEB") // case-insensitive substring of name
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, u.ID(), res[0].ID())
}

func testUserCaseInsensitiveEmailAlias(t *testing.T, nc Factory) {
	c, caps, done := nc(t)
	defer done()
	if !caps.CaseInsensitiveEmail {
		t.Skip("backend does not match email/alias case-insensitively")
	}
	ctx := context.Background()
	u, err := user.New().NewID().Name("ci").Email("Mixed@Example.com").Alias("MixedAlias").
		Workspace(id.NewWorkspaceID()).Build()
	require.NoError(t, err)
	require.NoError(t, c.User.Create(ctx, u))

	byEmail, err := c.User.FindByEmail(ctx, "mixed@example.COM")
	require.NoError(t, err)
	assert.Equal(t, u.ID(), byEmail.ID())

	byAlias, err := c.User.FindByAlias(ctx, "mixedalias")
	require.NoError(t, err)
	assert.Equal(t, u.ID(), byAlias.ID())
}

func testUserFindBySub(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	u, err := user.New().NewID().Name("b").Email("b@example.com").
		Workspace(id.NewWorkspaceID()).Auths([]user.Auth{user.AuthFrom("sub-xyz")}).Build()
	require.NoError(t, err)
	require.NoError(t, c.User.Create(ctx, u))
	got, err := c.User.FindBySub(ctx, "sub-xyz")
	require.NoError(t, err)
	assert.Equal(t, u.ID(), got.ID())
}

func testUserFindBySubOrCreate(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	u, err := user.New().NewID().Name("soc").Email("soc@example.com").
		Workspace(id.NewWorkspaceID()).Auths([]user.Auth{user.AuthFrom("sub-soc")}).Build()
	require.NoError(t, err)

	// first call creates
	created, err := c.User.FindBySubOrCreate(ctx, u, "sub-soc")
	require.NoError(t, err)
	assert.Equal(t, u.ID(), created.ID())

	// second call finds the existing one
	again, err := c.User.FindBySubOrCreate(ctx, newUser(t, "other", "other@example.com"), "sub-soc")
	require.NoError(t, err)
	assert.Equal(t, u.ID(), again.ID())
}

func testUserFindByVerification(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	v := user.VerificationFrom("code-123", timeFixed(), false)
	u, err := user.New().NewID().Name("v").Email("v@example.com").Workspace(id.NewWorkspaceID()).
		Verification(v).PasswordReset(&user.PasswordReset{Token: "tok-456", CreatedAt: timeFixed()}).Build()
	require.NoError(t, err)
	require.NoError(t, c.User.Create(ctx, u))

	byCode, err := c.User.FindByVerification(ctx, "code-123")
	require.NoError(t, err)
	assert.Equal(t, u.ID(), byCode.ID())

	byTok, err := c.User.FindByPasswordResetRequest(ctx, "tok-456")
	require.NoError(t, err)
	assert.Equal(t, u.ID(), byTok.ID())
}

func testUserDuplicateEmail(t *testing.T, nc Factory) {
	c, caps, done := nc(t)
	defer done()
	if !caps.UniqueEmail {
		t.Skip("backend does not enforce unique email")
	}
	ctx := context.Background()
	require.NoError(t, c.User.Create(ctx, newUser(t, "a", "dup@example.com")))
	err := c.User.Create(ctx, newUser(t, "b", "DUP@example.com")) // case-insensitive unique index
	assert.ErrorIs(t, err, user.ErrDuplicatedUser)
}

func testUserFindByIDsOrdering(t *testing.T, nc Factory) {
	c, caps, done := nc(t)
	defer done()
	if !caps.OrderedFindByIDs {
		t.Skip("backend does not preserve FindByIDs ordering with nil holes")
	}
	ctx := context.Background()
	a := newUser(t, "a", "a1@example.com")
	b := newUser(t, "b", "b1@example.com")
	require.NoError(t, c.User.Create(ctx, a))
	require.NoError(t, c.User.Create(ctx, b))
	missing := id.NewUserID()
	got, err := c.User.FindByIDs(ctx, user.IDList{b.ID(), missing, a.ID()})
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, b.ID(), got[0].ID())
	assert.Nil(t, got[1])
	assert.Equal(t, a.ID(), got[2].ID())
}

func testUserSearch(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	u, err := user.New().NewID().Name("Charlie Brown").Email("charlie@example.com").Workspace(id.NewWorkspaceID()).Build()
	require.NoError(t, err)
	require.NoError(t, c.User.Create(ctx, u))
	res, err := c.User.SearchByKeyword(ctx, "charl")
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, u.ID(), res[0].ID())
}

func testUserPagination(t *testing.T, nc Factory) {
	c, caps, done := nc(t)
	defer done()
	if !caps.RealPagination {
		t.Skip("backend does not implement real pagination")
	}
	ctx := context.Background()
	ids := user.IDList{}
	for i := 0; i < 5; i++ {
		u := newUser(t, "p", id.NewUserID().String()+"@example.com")
		require.NoError(t, c.User.Create(ctx, u))
		ids = append(ids, u.ID())
	}
	first := int64(2)
	list, info, err := c.User.FindByIDsWithPagination(ctx, ids, nil, usecasex.CursorPagination{First: &first}.Wrap())
	require.NoError(t, err)
	assert.Len(t, list, 2)
	assert.True(t, info.HasNextPage)
}

// ---- workspace ----

func testWorkspaceCRUD(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	uid := id.NewUserID()
	ws := newWorkspace(t, "team", uid)
	require.NoError(t, c.Workspace.Create(ctx, ws))
	got, err := c.Workspace.FindByID(ctx, ws.ID())
	require.NoError(t, err)
	assert.Equal(t, "team", got.Name())
	assert.Contains(t, got.Members().Users(), uid)

	_, err = c.Workspace.FindByID(ctx, id.NewWorkspaceID())
	assert.ErrorIs(t, err, rerror.ErrNotFound)
}

func testWorkspaceSaveUpdate(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	ws := newWorkspace(t, "before", id.NewUserID())
	require.NoError(t, c.Workspace.Create(ctx, ws))
	ws.Rename("after")
	require.NoError(t, c.Workspace.Save(ctx, ws))
	got, err := c.Workspace.FindByID(ctx, ws.ID())
	require.NoError(t, err)
	assert.Equal(t, "after", got.Name())
}

func testWorkspaceFindByNameAlias(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	uid := id.NewUserID()
	ws, err := workspace.New().NewID().Name("named-ws").Alias("wsalias").
		Members(map[id.UserID]workspace.Member{uid: {Role: role.RoleOwner, InvitedBy: uid}}).Build()
	require.NoError(t, err)
	require.NoError(t, c.Workspace.Create(ctx, ws))

	byName, err := c.Workspace.FindByName(ctx, "named-ws")
	require.NoError(t, err)
	assert.Equal(t, ws.ID(), byName.ID())

	byAlias, err := c.Workspace.FindByAlias(ctx, "wsalias")
	require.NoError(t, err)
	assert.Equal(t, ws.ID(), byAlias.ID())

	// empty string -> not found (mongo/memory/postgres all guard this)
	_, err = c.Workspace.FindByName(ctx, "")
	assert.ErrorIs(t, err, rerror.ErrNotFound)
}

func testWorkspaceFindByAliases(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	uid := id.NewUserID()
	ws, err := workspace.New().NewID().Name("a-ws").Alias("alias-a").
		Members(map[id.UserID]workspace.Member{uid: {Role: role.RoleOwner, InvitedBy: uid}}).Build()
	require.NoError(t, err)
	require.NoError(t, c.Workspace.Create(ctx, ws))

	list, err := c.Workspace.FindByAliases(ctx, []string{"alias-a", "missing-alias"})
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, ws.ID(), list[0].ID())
}

func testWorkspaceFindByIDs(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	uid := id.NewUserID()
	a := newWorkspace(t, "wa", uid)
	b := newWorkspace(t, "wb", uid)
	require.NoError(t, c.Workspace.Create(ctx, a))
	require.NoError(t, c.Workspace.Create(ctx, b))

	// found-only (a missing id is skipped, not nil-filled), set-equal
	got, err := c.Workspace.FindByIDs(ctx, id.WorkspaceIDList{a.ID(), id.NewWorkspaceID(), b.ID()})
	require.NoError(t, err)
	require.Len(t, got, 2)
	found := map[id.WorkspaceID]bool{}
	for _, w := range got {
		require.NotNil(t, w)
		found[w.ID()] = true
	}
	assert.True(t, found[a.ID()] && found[b.ID()])
}

func testWorkspaceFindByUser(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	uid := id.NewUserID()
	ws := newWorkspace(t, "w", uid)
	require.NoError(t, c.Workspace.Create(ctx, ws))
	list, err := c.Workspace.FindByUser(ctx, uid)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, ws.ID(), list[0].ID())
}

func testWorkspaceFindByIntegration(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	uid := id.NewUserID()
	iid := id.NewIntegrationID()
	ws, err := workspace.New().NewID().Name("wi").
		Members(map[id.UserID]workspace.Member{uid: {Role: role.RoleOwner, InvitedBy: uid}}).
		Integrations(map[id.IntegrationID]workspace.Member{iid: {Role: role.RoleOwner, InvitedBy: uid}}).Build()
	require.NoError(t, err)
	require.NoError(t, c.Workspace.Create(ctx, ws))

	one, err := c.Workspace.FindByIntegration(ctx, iid)
	require.NoError(t, err)
	require.Len(t, one, 1)
	assert.Equal(t, ws.ID(), one[0].ID())

	many, err := c.Workspace.FindByIntegrations(ctx, id.IntegrationIDList{iid})
	require.NoError(t, err)
	require.Len(t, many, 1)
	assert.Equal(t, ws.ID(), many[0].ID())
}

func testWorkspaceSaveAllRemoveAll(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	uid := id.NewUserID()
	a := newWorkspace(t, "sa", uid)
	b := newWorkspace(t, "sb", uid)
	require.NoError(t, c.Workspace.SaveAll(ctx, workspace.List{a, b}))

	got, err := c.Workspace.FindByIDs(ctx, id.WorkspaceIDList{a.ID(), b.ID()})
	require.NoError(t, err)
	assert.Len(t, got, 2)

	require.NoError(t, c.Workspace.RemoveAll(ctx, id.WorkspaceIDList{a.ID(), b.ID()}))
	_, err = c.Workspace.FindByID(ctx, a.ID())
	assert.ErrorIs(t, err, rerror.ErrNotFound)
}

func testWorkspaceRemove(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	ws := newWorkspace(t, "to-remove", id.NewUserID())
	require.NoError(t, c.Workspace.Create(ctx, ws))
	require.NoError(t, c.Workspace.Remove(ctx, ws.ID()))
	_, err := c.Workspace.FindByID(ctx, ws.ID())
	assert.ErrorIs(t, err, rerror.ErrNotFound)
}

func testWorkspaceFiltered(t *testing.T, nc Factory) {
	c, caps, done := nc(t)
	defer done()
	if !caps.EnforcesFilter {
		t.Skip("backend does not enforce workspace filter")
	}
	ctx := context.Background()
	uid := id.NewUserID()
	visible := newWorkspace(t, "v", uid)
	hidden := newWorkspace(t, "h", uid)
	require.NoError(t, c.Workspace.Create(ctx, visible))
	require.NoError(t, c.Workspace.Create(ctx, hidden))

	// constrain both read and write to `visible` so `hidden` is neither readable
	// (CanRead falls through to CanWrite) nor writable.
	f := c.Workspace.Filtered(workspace.WorkspaceFilter{
		Readable: id.WorkspaceIDList{visible.ID()},
		Writable: id.WorkspaceIDList{visible.ID()},
	})

	_, err := f.FindByIDs(ctx, id.WorkspaceIDList{hidden.ID()})
	assert.Error(t, err)

	got, err := f.FindByIDs(ctx, id.WorkspaceIDList{visible.ID()})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, visible.ID(), got[0].ID())

	assert.Error(t, f.Save(ctx, hidden))
	assert.NoError(t, f.Save(ctx, visible))
}

// ---- role ----

func testRoleCRUD(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	rl, err := role.New().NewID().Name("admin").Build()
	require.NoError(t, err)
	require.NoError(t, c.Role.Save(ctx, *rl))
	got, err := c.Role.FindByName(ctx, "admin")
	require.NoError(t, err)
	assert.Equal(t, rl.ID(), got.ID())
	byID, err := c.Role.FindByID(ctx, rl.ID())
	require.NoError(t, err)
	assert.Equal(t, "admin", byID.Name())
	require.NoError(t, c.Role.Remove(ctx, rl.ID()))
	_, err = c.Role.FindByID(ctx, rl.ID())
	assert.ErrorIs(t, err, rerror.ErrNotFound)
}

func testRoleFindAllAndByIDs(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	r1, _ := role.New().NewID().Name("r-one").Build()
	r2, _ := role.New().NewID().Name("r-two").Build()
	require.NoError(t, c.Role.Save(ctx, *r1))
	require.NoError(t, c.Role.Save(ctx, *r2))

	all, err := c.Role.FindAll(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(all), 2)

	// found-only (a missing id is skipped), set-equal
	got, err := c.Role.FindByIDs(ctx, id.RoleIDList{r1.ID(), id.NewRoleID(), r2.ID()})
	require.NoError(t, err)
	require.Len(t, got, 2)
	found := map[id.RoleID]bool{}
	for _, r := range got {
		require.NotNil(t, r)
		found[r.ID()] = true
	}
	assert.True(t, found[r1.ID()] && found[r2.ID()])
}

// ---- permittable ----

func newPermittable(t *testing.T, uid id.UserID, rids ...id.RoleID) permittable.Permittable {
	t.Helper()
	p, err := permittable.New().NewID().UserID(uid).RoleIDs(rids).Build()
	require.NoError(t, err)
	return *p
}

func testPermittable(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	uid := id.NewUserID()
	rid := id.NewRoleID()
	require.NoError(t, c.Permittable.Save(ctx, newPermittable(t, uid, rid)))

	got, err := c.Permittable.FindByUserID(ctx, uid)
	require.NoError(t, err)
	assert.Equal(t, uid, got.UserID())

	byRole, err := c.Permittable.FindByRoleID(ctx, rid)
	require.NoError(t, err)
	require.Len(t, byRole, 1)
	assert.Equal(t, uid, byRole[0].UserID())
}

func testPermittableWorkspaceRoles(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	uid := id.NewUserID()
	rid := id.NewRoleID()
	wid := id.NewWorkspaceID()
	p, err := permittable.New().NewID().UserID(uid).RoleIDs([]id.RoleID{rid}).
		WorkspaceRoles([]permittable.WorkspaceRole{permittable.NewWorkspaceRole(wid, rid)}).Build()
	require.NoError(t, err)
	require.NoError(t, c.Permittable.Save(ctx, *p))

	got, err := c.Permittable.FindByUserID(ctx, uid)
	require.NoError(t, err)
	require.Len(t, got.WorkspaceRoles(), 1)
	wr := got.WorkspaceRoles()[0]
	assert.Equal(t, wid, wr.ID())
	assert.Equal(t, rid, wr.RoleID())
}

func testPermittableFindByUserIDsAndSaveMany(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	u1, u2 := id.NewUserID(), id.NewUserID()
	p1 := newPermittable(t, u1, id.NewRoleID())
	p2 := newPermittable(t, u2, id.NewRoleID())
	require.NoError(t, c.Permittable.SaveMany(ctx, permittable.List{&p1, &p2}))

	got, err := c.Permittable.FindByUserIDs(ctx, user.IDList{u1, u2})
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func testPermittableNotFound(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	_, err := c.Permittable.FindByUserID(ctx, id.NewUserID())
	assert.ErrorIs(t, err, rerror.ErrNotFound)
	// plural queries also return ErrNotFound on an empty result (mongo parity)
	_, err = c.Permittable.FindByRoleID(ctx, id.NewRoleID())
	assert.ErrorIs(t, err, rerror.ErrNotFound)
}

// ---- config ----

func testConfig(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	cfg, err := c.Config.LockAndLoad(ctx)
	require.NoError(t, err)
	if cfg == nil {
		cfg = &config.Config{}
	}
	cfg.Migration = 7
	require.NoError(t, c.Config.SaveAndUnlock(ctx, cfg))

	reloaded, err := c.Config.LockAndLoad(ctx)
	require.NoError(t, err)
	require.NotNil(t, reloaded)
	assert.Equal(t, int64(7), reloaded.Migration)
	require.NoError(t, c.Config.Unlock(ctx))
}

func testConfigSaveAuth(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	require.NoError(t, c.Config.SaveAuth(ctx, &config.Auth{Cert: "cert-x", Key: "key-y"}))

	cfg, err := c.Config.LockAndLoad(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.Auth)
	assert.Equal(t, "cert-x", cfg.Auth.Cert)
	assert.Equal(t, "key-y", cfg.Auth.Key)
	require.NoError(t, c.Config.Unlock(ctx))
}

// ---- transaction ----

// testTransactorWiring asserts every backend exposes a non-nil Transactor and a
// no-op closure round-trips. Behavior parity with usecasex.Transaction is
// already covered by testTransaction; this is the wiring smoke check.
func testTransactorWiring(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	require.NotNil(t, c.Transactor, "repo.Container.Transactor must be wired")
	require.NoError(t, c.Transactor.WithinTransaction(context.Background(), func(ctx context.Context) error { return nil }))
}

func testTransaction(t *testing.T, nc Factory) {
	c, caps, done := nc(t)
	defer done()
	ctx := context.Background()
	uid := id.NewUserID()
	_ = usecasex.DoTransaction(ctx, c.Transaction, 0, func(ctx context.Context) error {
		u, err := user.New().ID(uid).Name("tx").Email("tx@example.com").Workspace(id.NewWorkspaceID()).Build()
		require.NoError(t, err)
		require.NoError(t, c.User.Create(ctx, u))
		return assert.AnError // force rollback
	})
	_, err := c.User.FindByID(ctx, uid)
	if caps.RealTransactions {
		assert.ErrorIs(t, err, rerror.ErrNotFound) // rolled back -> not found
	}
}
