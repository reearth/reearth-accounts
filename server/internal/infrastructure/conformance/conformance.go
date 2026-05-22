// Package conformance holds a single, parameterized repository test suite that
// every persistence backend (memory, mongo, postgres) must satisfy. Backends
// declare which optional behaviors they support via Caps so the shared suite can
// gate backend-specific assertions instead of forking per backend.
package conformance

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/config"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/usecasex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Caps describes optional behaviors a backend may or may not implement.
type Caps struct {
	RealTransactions     bool // Begin/Commit/Rollback actually isolate writes
	EnforcesFilter       bool // Filtered() enforces read/write permissions
	OrderedFindByIDs     bool // FindByIDs preserves request order with nil holes
	RealPagination       bool // cursor/offset pagination is honored
	CaseInsensitiveEmail bool // FindByEmail is case-insensitive
	UniqueEmail          bool // duplicate emails are rejected on Create
}

// Factory returns a fresh, empty repo.Container, its capabilities, and cleanup.
type Factory func(t *testing.T) (*repo.Container, Caps, func())

// Run executes the full conformance suite against the given backend factory.
func Run(t *testing.T, nc Factory) {
	t.Run("User_CRUD", func(t *testing.T) { testUserCRUD(t, nc) })
	t.Run("User_CaseInsensitiveEmail", func(t *testing.T) { testUserCaseInsensitiveEmail(t, nc) })
	t.Run("User_FindBySub", func(t *testing.T) { testUserFindBySub(t, nc) })
	t.Run("User_DuplicateEmail", func(t *testing.T) { testUserDuplicateEmail(t, nc) })
	t.Run("User_FindByIDs_Ordering", func(t *testing.T) { testUserFindByIDsOrdering(t, nc) })
	t.Run("User_SearchByKeyword", func(t *testing.T) { testUserSearch(t, nc) })
	t.Run("User_Pagination", func(t *testing.T) { testUserPagination(t, nc) })
	t.Run("Workspace_CRUD_Members", func(t *testing.T) { testWorkspaceCRUD(t, nc) })
	t.Run("Workspace_FindByUser", func(t *testing.T) { testWorkspaceFindByUser(t, nc) })
	t.Run("Workspace_Filtered", func(t *testing.T) { testWorkspaceFiltered(t, nc) })
	t.Run("Role_CRUD", func(t *testing.T) { testRoleCRUD(t, nc) })
	t.Run("Permittable_RoleQueries", func(t *testing.T) { testPermittable(t, nc) })
	t.Run("Config_LockLoadSave", func(t *testing.T) { testConfig(t, nc) })
	t.Run("Transaction_CommitRollback", func(t *testing.T) { testTransaction(t, nc) })
}

func newUser(t *testing.T, name, email string) *user.User {
	t.Helper()
	u, err := user.New().NewID().Name(name).Email(email).Workspace(id.NewWorkspaceID()).Build()
	require.NoError(t, err)
	return u
}

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

	require.NoError(t, c.User.Remove(ctx, uid))
	_, err = c.User.FindByID(ctx, uid)
	assert.Error(t, err)
}

func testUserCaseInsensitiveEmail(t *testing.T, nc Factory) {
	c, caps, done := nc(t)
	defer done()
	if !caps.CaseInsensitiveEmail {
		t.Skip("backend does not provide case-insensitive email lookup")
	}
	ctx := context.Background()
	u := newUser(t, "ci", "Mixed@Example.com")
	require.NoError(t, c.User.Create(ctx, u))
	got, err := c.User.FindByEmail(ctx, "mixed@example.COM")
	require.NoError(t, err)
	assert.Equal(t, u.ID(), got.ID())
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

func testUserDuplicateEmail(t *testing.T, nc Factory) {
	c, caps, done := nc(t)
	defer done()
	if !caps.UniqueEmail {
		t.Skip("backend does not enforce unique email")
	}
	ctx := context.Background()
	require.NoError(t, c.User.Create(ctx, newUser(t, "a", "dup@example.com")))
	err := c.User.Create(ctx, newUser(t, "b", "DUP@example.com"))
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

func newWorkspace(t *testing.T, name string, owner id.UserID) *workspace.Workspace {
	t.Helper()
	ws, err := workspace.New().NewID().Name(name).
		Members(map[id.UserID]workspace.Member{owner: {Role: role.RoleOwner, InvitedBy: owner}}).Build()
	require.NoError(t, err)
	return ws
}

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

	// constrain both read and write to `visible` so `hidden` is neither
	// readable (CanRead falls through to CanWrite) nor writable.
	f := c.Workspace.Filtered(workspace.WorkspaceFilter{
		Readable: id.WorkspaceIDList{visible.ID()},
		Writable: id.WorkspaceIDList{visible.ID()},
	})

	_, err := f.FindByIDs(ctx, id.WorkspaceIDList{hidden.ID()})
	assert.Error(t, err) // not readable

	got, err := f.FindByIDs(ctx, id.WorkspaceIDList{visible.ID()})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, visible.ID(), got[0].ID())

	assert.Error(t, f.Save(ctx, hidden))    // not writable
	assert.NoError(t, f.Save(ctx, visible)) // writable
}

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
	require.NoError(t, c.Role.Remove(ctx, rl.ID()))
	_, err = c.Role.FindByID(ctx, rl.ID())
	assert.Error(t, err)
}

func testPermittable(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	uid := id.NewUserID()
	rid := id.NewRoleID()
	p, err := permittable.New().NewID().UserID(uid).RoleIDs([]id.RoleID{rid}).Build()
	require.NoError(t, err)
	require.NoError(t, c.Permittable.Save(ctx, *p))

	got, err := c.Permittable.FindByUserID(ctx, uid)
	require.NoError(t, err)
	assert.Equal(t, uid, got.UserID())

	byRole, err := c.Permittable.FindByRoleID(ctx, rid)
	require.NoError(t, err)
	require.Len(t, byRole, 1)
	assert.Equal(t, uid, byRole[0].UserID())
}

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
		assert.Error(t, err) // rolled back -> not found
	}
}
