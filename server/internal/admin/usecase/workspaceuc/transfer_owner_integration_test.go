//go:build integration

package workspaceuc

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres/migration"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
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

func newPostgresRepos(t *testing.T) *repo.Container {
	t.Helper()
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

	// Role records come from the schema migration above.
	repos, err := postgres.New(ctx, pool, nil)
	require.NoError(t, err)
	return repos
}

func savePostgresUser(t *testing.T, repos *repo.Container, name string) *user.User {
	t.Helper()
	// The postgres user row requires a personal workspace id.
	u := user.New().NewID().Name(name).Email(name + "@example.com").Workspace(workspace.NewID()).MustBuild()
	require.NoError(t, repos.User.Save(context.Background(), u))
	return u
}

// A Save failure after the permittable writes must roll them back with the
// role changes, which only a real transaction can show.
func TestTransferOwner_Postgres_SaveFailureRollsBackPermittables(t *testing.T) {
	ctx := context.Background()
	repos := newPostgresRepos(t)
	a := savePostgresUser(t, repos, "a")
	b := savePostgresUser(t, repos, "b")
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
	_, err := uc.Execute(ctx, TransferOwnerInput{WorkspaceID: ws.ID(), NewOwnerID: b.ID()})
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

// lockProbeRepo reports when a transfer has taken the workspace lock and holds
// the first Save until release is closed, so a second transfer can be started
// while the first one still holds the lock.
type lockProbeRepo struct {
	workspace.Repo
	locked  chan struct{}
	inSave  chan struct{}
	release chan struct{}
	saves   chan struct{}
}

func (r *lockProbeRepo) FindByIDForUpdate(ctx context.Context, id workspace.ID) (*workspace.Workspace, error) {
	ws, err := r.Repo.FindByIDForUpdate(ctx, id)
	r.locked <- struct{}{}
	return ws, err
}

func (r *lockProbeRepo) Save(ctx context.Context, ws *workspace.Workspace) error {
	select {
	case r.saves <- struct{}{}:
		r.inSave <- struct{}{}
		<-r.release
	default:
	}
	return r.Repo.Save(ctx, ws)
}

// Two transfers on one workspace must run one after the other: the second
// must read the members the first committed, or the first target keeps an
// owner permittable while its member role is overwritten.
func TestTransferOwner_Postgres_ConcurrentTransfersAreSerialized(t *testing.T) {
	ctx := context.Background()
	repos := newPostgresRepos(t)
	o := savePostgresUser(t, repos, "o")
	x := savePostgresUser(t, repos, "x")
	y := savePostgresUser(t, repos, "y")
	ws := teamWs(map[workspace.UserID]workspace.Member{
		o.ID(): {Role: role.RoleOwner},
		x.ID(): {Role: role.RoleWriter},
		y.ID(): {Role: role.RoleWriter},
	})
	require.NoError(t, repos.Workspace.Save(ctx, ws))

	probe := &lockProbeRepo{
		Repo:    repos.Workspace,
		locked:  make(chan struct{}, 2),
		inSave:  make(chan struct{}, 1),
		release: make(chan struct{}),
		saves:   make(chan struct{}, 1),
	}
	release := sync.OnceFunc(func() { close(probe.release) })
	// Runs before the pool is closed, so a failed assertion cannot leave the
	// first transfer holding a connection forever.
	t.Cleanup(release)
	uc := NewTransferOwnerUseCase(probe, repos.User, repos.Role, repos.Permittable, repos.Transaction)

	errs := make(chan error, 2)
	go func() {
		_, err := uc.Execute(ctx, TransferOwnerInput{WorkspaceID: ws.ID(), NewOwnerID: x.ID()})
		errs <- err
	}()
	<-probe.locked
	<-probe.inSave

	go func() {
		_, err := uc.Execute(ctx, TransferOwnerInput{WorkspaceID: ws.ID(), NewOwnerID: y.ID()})
		errs <- err
	}()
	select {
	case <-probe.locked:
		t.Fatal("second transfer read the workspace while the first held the lock")
	case <-time.After(500 * time.Millisecond):
	}

	release()
	require.NoError(t, <-errs)
	require.NoError(t, <-errs)

	saved, err := repos.Workspace.FindByID(ctx, ws.ID())
	require.NoError(t, err)
	want := map[workspace.UserID]role.RoleType{
		o.ID(): role.RoleMaintainer,
		x.ID(): role.RoleMaintainer,
		y.ID(): role.RoleOwner,
	}
	for uid, rt := range want {
		assert.Equal(t, rt, saved.Members().UserRole(uid), "member role of %s", uid)

		r, err := repos.Role.FindByName(ctx, string(rt))
		require.NoError(t, err)
		p, err := repos.Permittable.FindByUserID(ctx, uid)
		require.NoError(t, err)
		var got []string
		for _, wr := range p.WorkspaceRoles() {
			if wr.ID() == ws.ID() {
				got = append(got, wr.RoleID().String())
			}
		}
		assert.Equal(t, []string{r.ID().String()}, got, "permittable role of %s", uid)
	}
}
