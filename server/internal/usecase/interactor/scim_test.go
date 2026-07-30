package interactor

import (
	"context"
	"testing"

	accountmemory "github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newScimEnabledWorkspace(ownerID user.ID) *workspace.Workspace {
	cfg := workspace.NewScimConfig()
	cfg.SetEnabled(true)
	return workspace.New().
		NewID().
		Name("enterprise").
		Alias("enterprise").
		Members(map[user.ID]workspace.Member{
			ownerID: {Role: role.RoleOwner, InvitedBy: ownerID},
		}).
		ScimConfig(cfg).
		MustBuild()
}

func TestProvisionScimUser_NewUser(t *testing.T) {
	ctx := context.Background()
	db := accountmemory.New()

	ownerID := user.NewID()
	owner := user.New().ID(ownerID).Name("owner").Email("owner@example.com").Workspace(user.NewWorkspaceID()).MustBuild()
	require.NoError(t, db.User.Save(ctx, owner))

	ws := newScimEnabledWorkspace(ownerID)
	require.NoError(t, db.Workspace.Save(ctx, ws))

	uc := NewScim(db)

	u, err := uc.ProvisionScimUser(ctx, interfaces.ProvisionScimUserParam{
		Email:       "alice@example.com",
		ExternalID:  "ext-alice-001",
		Name:        "Alice",
		Role:        role.RoleWriter,
		WorkspaceID: ws.ID(),
	})

	require.NoError(t, err)
	require.NotNil(t, u)
	assert.Equal(t, "alice@example.com", u.Email())
	assert.Equal(t, "Alice", u.Name())

	// verify membership with ExternalID set
	saved, err := db.Workspace.FindByID(ctx, ws.ID())
	require.NoError(t, err)
	uid, ok := saved.Members().UserByExternalID("ext-alice-001")
	assert.True(t, ok)
	assert.Equal(t, u.ID(), uid)
}

func TestProvisionScimUser_Idempotent(t *testing.T) {
	ctx := context.Background()
	db := accountmemory.New()

	ownerID := user.NewID()
	owner := user.New().ID(ownerID).Name("owner").Email("owner@example.com").Workspace(user.NewWorkspaceID()).MustBuild()
	require.NoError(t, db.User.Save(ctx, owner))

	ws := newScimEnabledWorkspace(ownerID)
	require.NoError(t, db.Workspace.Save(ctx, ws))

	uc := NewScim(db)

	param := interfaces.ProvisionScimUserParam{
		Email:       "bob@example.com",
		ExternalID:  "ext-bob-001",
		Name:        "Bob",
		WorkspaceID: ws.ID(),
	}

	u1, err := uc.ProvisionScimUser(ctx, param)
	require.NoError(t, err)
	require.NotNil(t, u1)

	u2, err := uc.ProvisionScimUser(ctx, param)
	require.NoError(t, err)
	require.NotNil(t, u2)

	assert.Equal(t, u1.ID(), u2.ID(), "second call must return the same user, not a duplicate")

	// verify only one membership with this ExternalID exists
	saved, err := db.Workspace.FindByID(ctx, ws.ID())
	require.NoError(t, err)
	count := 0
	for _, m := range saved.Members().Users() {
		if m.ExternalID == "ext-bob-001" {
			count++
		}
	}
	assert.Equal(t, 1, count)
}

func TestProvisionScimUser_ExistingUserByEmail(t *testing.T) {
	ctx := context.Background()
	db := accountmemory.New()

	ownerID := user.NewID()
	owner := user.New().ID(ownerID).Name("owner").Email("owner@example.com").Workspace(user.NewWorkspaceID()).MustBuild()
	require.NoError(t, db.User.Save(ctx, owner))

	ws := newScimEnabledWorkspace(ownerID)
	require.NoError(t, db.Workspace.Save(ctx, ws))

	// Pre-existing user (e.g. joined via JIT)
	existingID := user.NewID()
	existing := user.New().ID(existingID).Name("Carol").Email("carol@example.com").Workspace(user.NewWorkspaceID()).MustBuild()
	require.NoError(t, db.User.Save(ctx, existing))

	uc := NewScim(db)

	u, err := uc.ProvisionScimUser(ctx, interfaces.ProvisionScimUserParam{
		Email:       "carol@example.com",
		ExternalID:  "ext-carol-001",
		Name:        "Carol",
		WorkspaceID: ws.ID(),
	})

	require.NoError(t, err)
	require.NotNil(t, u)
	assert.Equal(t, existingID, u.ID(), "must reuse existing user, not create a new one")

	saved, err := db.Workspace.FindByID(ctx, ws.ID())
	require.NoError(t, err)
	assert.True(t, saved.Members().HasUser(existingID))
}

func TestDeprovisionScimUser_OK(t *testing.T) {
	ctx := context.Background()
	db := accountmemory.New()

	ownerID := user.NewID()
	owner := user.New().ID(ownerID).Name("owner").Email("owner@example.com").Workspace(user.NewWorkspaceID()).MustBuild()
	require.NoError(t, db.User.Save(ctx, owner))

	memberID := user.NewID()
	member := user.New().ID(memberID).Name("member").Email("member@example.com").Workspace(user.NewWorkspaceID()).MustBuild()
	require.NoError(t, db.User.Save(ctx, member))

	cfg := workspace.NewScimConfig()
	cfg.SetEnabled(true)
	ws := workspace.New().
		NewID().
		Name("enterprise").
		Alias("enterprise").
		Members(map[user.ID]workspace.Member{
			ownerID:  {Role: role.RoleOwner, InvitedBy: ownerID},
			memberID: {Role: role.RoleWriter, InvitedBy: ownerID, ExternalID: "ext-member-001"},
		}).
		ScimConfig(cfg).
		MustBuild()
	require.NoError(t, db.Workspace.Save(ctx, ws))

	uc := NewScim(db)

	err := uc.DeprovisionScimUser(ctx, ws.ID(), "ext-member-001")
	require.NoError(t, err)

	saved, err := db.Workspace.FindByID(ctx, ws.ID())
	require.NoError(t, err)

	m := saved.Members().User(memberID)
	require.NotNil(t, m)
	assert.True(t, m.Disabled, "member must be soft-disabled, not removed")
}

func TestDeprovisionScimUser_LastOwner(t *testing.T) {
	ctx := context.Background()
	db := accountmemory.New()

	ownerID := user.NewID()
	owner := user.New().ID(ownerID).Name("owner").Email("owner@example.com").Workspace(user.NewWorkspaceID()).MustBuild()
	require.NoError(t, db.User.Save(ctx, owner))

	cfg := workspace.NewScimConfig()
	cfg.SetEnabled(true)
	ws := workspace.New().
		NewID().
		Name("enterprise").
		Alias("enterprise").
		Members(map[user.ID]workspace.Member{
			ownerID: {Role: role.RoleOwner, InvitedBy: ownerID, ExternalID: "ext-owner-001"},
		}).
		ScimConfig(cfg).
		MustBuild()
	require.NoError(t, db.Workspace.Save(ctx, ws))

	uc := NewScim(db)

	err := uc.DeprovisionScimUser(ctx, ws.ID(), "ext-owner-001")
	assert.ErrorIs(t, err, interfaces.ErrOwnerCannotLeaveTheWorkspace)
}

func TestSyncScimGroup_AddAndRemove(t *testing.T) {
	ctx := context.Background()
	db := accountmemory.New()

	ownerID := user.NewID()
	owner := user.New().ID(ownerID).Name("owner").Email("owner@example.com").Workspace(user.NewWorkspaceID()).MustBuild()
	require.NoError(t, db.User.Save(ctx, owner))

	// A user who will be removed
	removedID := user.NewID()
	removed := user.New().ID(removedID).Name("removed").Email("removed@example.com").Workspace(user.NewWorkspaceID()).MustBuild()
	require.NoError(t, db.User.Save(ctx, removed))

	// A new user to be added
	newID := user.NewID()
	newUser := user.New().ID(newID).Name("newuser").Email("new@example.com").Workspace(user.NewWorkspaceID()).MustBuild()
	require.NoError(t, db.User.Save(ctx, newUser))

	cfg := workspace.NewScimConfig()
	cfg.SetEnabled(true)
	ws := workspace.New().
		NewID().
		Name("enterprise").
		Alias("enterprise").
		Members(map[user.ID]workspace.Member{
			ownerID:   {Role: role.RoleOwner, InvitedBy: ownerID},
			removedID: {Role: role.RoleWriter, InvitedBy: ownerID, ExternalID: "ext-removed-001"},
		}).
		ScimConfig(cfg).
		MustBuild()
	require.NoError(t, db.Workspace.Save(ctx, ws))

	uc := NewScim(db)

	err := uc.SyncScimGroup(ctx, ws.ID(), "group-id", "engineers", []interfaces.ScimGroupMember{
		{ExternalID: "ext-new-001", UserID: &newID},
	})
	require.NoError(t, err)

	saved, err := db.Workspace.FindByID(ctx, ws.ID())
	require.NoError(t, err)

	// new user must be a member
	assert.True(t, saved.Members().HasUser(newID))

	// removed user must be soft-disabled
	m := saved.Members().User(removedID)
	require.NotNil(t, m)
	assert.True(t, m.Disabled)
}

func TestGenerateScimToken(t *testing.T) {
	ctx := context.Background()
	db := accountmemory.New()

	ownerID := user.NewID()
	owner := user.New().ID(ownerID).Name("owner").Email("owner@example.com").Workspace(user.NewWorkspaceID()).MustBuild()
	require.NoError(t, db.User.Save(ctx, owner))

	ws := newScimEnabledWorkspace(ownerID)
	require.NoError(t, db.Workspace.Save(ctx, ws))

	op := &workspace.Operator{
		User:                   &ownerID,
		MaintainableWorkspaces: []workspace.ID{ws.ID()},
	}

	uc := NewScim(db)

	plaintext1, err := uc.GenerateScimToken(ctx, ws.ID(), op)
	require.NoError(t, err)
	assert.NotEmpty(t, plaintext1)

	saved, err := db.Workspace.FindByID(ctx, ws.ID())
	require.NoError(t, err)
	hash1 := saved.ScimConfig().TokenHash()
	assert.NotEmpty(t, hash1)

	// second call replaces hash
	plaintext2, err := uc.GenerateScimToken(ctx, ws.ID(), op)
	require.NoError(t, err)
	assert.NotEmpty(t, plaintext2)
	assert.NotEqual(t, plaintext1, plaintext2)

	saved2, err := db.Workspace.FindByID(ctx, ws.ID())
	require.NoError(t, err)
	hash2 := saved2.ScimConfig().TokenHash()
	assert.NotEmpty(t, hash2)
	assert.NotEqual(t, hash1, hash2)
}
