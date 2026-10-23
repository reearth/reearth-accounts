package bootstrap

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func roleNames(t *testing.T, r role.Repo) []string {
	t.Helper()
	all, err := r.FindAll(context.Background())
	require.NoError(t, err)
	names := make([]string, 0, len(all))
	for _, rl := range all {
		names = append(names, rl.Name())
	}
	return names
}

func roleIDs(t *testing.T, r role.Repo) []role.ID {
	t.Helper()
	all, err := r.FindAll(context.Background())
	require.NoError(t, err)
	ids := make([]role.ID, 0, len(all))
	for _, rl := range all {
		ids = append(ids, rl.ID())
	}
	return ids
}

func allRoleNames() []string {
	names := []string{}
	for _, rt := range role.RoleTypes() {
		names = append(names, rt.String())
	}
	return names
}

func TestSeedMockAuthRoles_CreatesMissingRoles(t *testing.T) {
	ctx := context.Background()
	existing := role.New().NewID().Name(role.RoleOwner.String()).MustBuild()
	r := memory.NewRoleWith(existing)

	require.NoError(t, SeedMockAuthRoles(ctx, true, r))

	assert.ElementsMatch(t, allRoleNames(), roleNames(t, r))
	owner, err := r.FindByName(ctx, role.RoleOwner.String())
	require.NoError(t, err)
	assert.Equal(t, existing.ID(), owner.ID(), "an existing role record must be kept")
}

func TestSeedMockAuthRoles_Idempotent(t *testing.T) {
	ctx := context.Background()
	r := memory.NewRole()

	require.NoError(t, SeedMockAuthRoles(ctx, true, r))
	first := roleIDs(t, r)

	require.NoError(t, SeedMockAuthRoles(ctx, true, r))

	assert.ElementsMatch(t, first, roleIDs(t, r))
	assert.ElementsMatch(t, allRoleNames(), roleNames(t, r))
}

func TestSeedMockAuthRoles_DisabledIsNoop(t *testing.T) {
	r := memory.NewRole()

	require.NoError(t, SeedMockAuthRoles(context.Background(), false, r))

	assert.Empty(t, roleNames(t, r))
}
