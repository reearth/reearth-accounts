//go:build integration

package workspaceuc

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres/migration"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/rerror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// A Save failure after the permittable writes must roll them back with the
// role changes, which only a real transaction can show.
func TestTransferOwner_Postgres_SaveFailureRollsBackPermittables(t *testing.T) {
	ctx := context.Background()

	c, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("test"), tcpostgres.WithUsername("test"), tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2)))
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Terminate(ctx) })

	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, migration.Migrate(ctx, pool))

	repos, err := postgres.New(ctx, pool, nil)
	require.NoError(t, err)

	for _, rt := range []role.RoleType{role.RoleOwner, role.RoleMaintainer} {
		require.NoError(t, repos.Role.Save(ctx, *role.New().NewID().Name(string(rt)).MustBuild()))
	}
	// The postgres user row requires a personal workspace id.
	a := user.New().NewID().Name("a").Email("a@example.com").Workspace(workspace.NewID()).MustBuild()
	b := user.New().NewID().Name("b").Email("b@example.com").Workspace(workspace.NewID()).MustBuild()
	require.NoError(t, repos.User.Save(ctx, a))
	require.NoError(t, repos.User.Save(ctx, b))
	ws := teamWs(map[workspace.UserID]workspace.Member{
		a.ID(): {Role: role.RoleOwner},
		b.ID(): {Role: role.RoleWriter},
	})
	require.NoError(t, repos.Workspace.Save(ctx, ws))

	saveErr := errors.New("save failed")
	uc := NewTransferOwnerUseCase(
		failingSaveWorkspaceRepo{Repo: repos.Workspace, err: saveErr},
		repos.User, repos.Role, repos.Permittable, repos.Transaction,
	)
	_, err = uc.Execute(ctx, TransferOwnerInput{WorkspaceID: ws.ID(), NewOwnerID: b.ID()})
	require.ErrorIs(t, err, saveErr)

	for _, u := range []workspace.UserID{a.ID(), b.ID()} {
		_, err := repos.Permittable.FindByUserID(ctx, u)
		assert.ErrorIs(t, err, rerror.ErrNotFound)
	}
	saved, err := repos.Workspace.FindByID(ctx, ws.ID())
	require.NoError(t, err)
	assert.Equal(t, role.RoleOwner, saved.Members().UserRole(a.ID()))
	assert.Equal(t, role.RoleWriter, saved.Members().UserRole(b.ID()))

	uc = NewTransferOwnerUseCase(repos.Workspace, repos.User, repos.Role, repos.Permittable, repos.Transaction)
	_, err = uc.Execute(ctx, TransferOwnerInput{WorkspaceID: ws.ID(), NewOwnerID: b.ID()})
	require.NoError(t, err)
	saved, err = repos.Workspace.FindByID(ctx, ws.ID())
	require.NoError(t, err)
	assert.Equal(t, role.RoleOwner, saved.Members().UserRole(b.ID()))
	assert.Equal(t, role.RoleMaintainer, saved.Members().UserRole(a.ID()))
	_, err = repos.Permittable.FindByUserID(ctx, a.ID())
	assert.NoError(t, err)
}
