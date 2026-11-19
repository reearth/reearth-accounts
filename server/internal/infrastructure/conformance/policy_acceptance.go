package conformance

import (
	"context"
	"fmt"
	"net/netip"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interactor"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/legal"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/rerror"
	"github.com/reearth/reearthx/usecasex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runPolicyAcceptance(t *testing.T, nc Factory) {
	t.Run("PolicyAcceptance_InsertIfAbsent", func(t *testing.T) { testPolicyAcceptanceInsertIfAbsent(t, nc) })
	t.Run("PolicyAcceptance_InsertRace", func(t *testing.T) { testPolicyAcceptanceInsertRace(t, nc) })
	t.Run("PolicyAcceptance_Remove", func(t *testing.T) { testPolicyAcceptanceRemove(t, nc) })
	t.Run("PolicyAcceptance_RemoveByUser", func(t *testing.T) { testPolicyAcceptanceRemoveByUser(t, nc) })
	t.Run("PolicyAcceptance_RemoveRollsBack", func(t *testing.T) { testPolicyAcceptanceRemoveRollsBack(t, nc) })
	t.Run("PolicyAcceptance_AcceptRacingDeleteMe", func(t *testing.T) { testPolicyAcceptanceAcceptRacingDeleteMe(t, nc) })
	t.Run("PolicyAcceptance_AcceptCutOffAfterDeleteMe", func(t *testing.T) { testPolicyAcceptanceAcceptCutOffAfterDeleteMe(t, nc) })
	t.Run("PolicyAcceptance_AcceptReReadFails", func(t *testing.T) { testPolicyAcceptanceAcceptReReadFails(t, nc) })
	t.Run("PolicyAcceptance_DeleteMeSweepCutOff", func(t *testing.T) { testPolicyAcceptanceDeleteMeSweepCutOff(t, nc) })
}

func acceptance(u id.UserID, typ, version, ua string) legal.Acceptance {
	return legal.Acceptance{
		AcceptedAt: timeFixed().Truncate(time.Millisecond),
		Client: legal.Client{
			IP:        netip.MustParseAddr("203.0.113.7"),
			Origin:    "https://app.example.com",
			UserAgent: ua,
		},
		Ref:    legal.Ref{Type: legal.Type(typ), Version: legal.MustParseVersion(version)},
		UserID: u,
	}
}

func acceptancesOf(t *testing.T, r legal.AcceptanceRepo, u id.UserID) []legal.Acceptance {
	t.Helper()
	as, err := r.FindByUser(context.Background(), u)
	require.NoError(t, err)
	sort.Slice(as, func(i, j int) bool {
		if as[i].Type != as[j].Type {
			return as[i].Type < as[j].Type
		}
		return as[i].Version.Compare(as[j].Version) < 0
	})
	return as
}

func testPolicyAcceptanceInsertIfAbsent(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	r := c.PolicyAcceptance
	u := id.NewUserID()

	assert.Empty(t, acceptancesOf(t, r, u))
	require.NoError(t, r.InsertIfAbsent(ctx, nil))

	first := acceptance(u, "tos", "1.0.0", "first")
	require.NoError(t, r.InsertIfAbsent(ctx, []legal.Acceptance{first}))
	assert.Equal(t, []legal.Acceptance{first}, acceptancesOf(t, r, u))

	noIP := acceptance(u, "privacy", "1.0.0", "second")
	noIP.Client.IP = netip.Addr{}
	require.NoError(t, r.InsertIfAbsent(ctx, []legal.Acceptance{
		acceptance(u, "tos", "1.0.0", "second"),
		noIP,
		acceptance(u, "tos", "2.0.0", "second"),
	}))
	assert.Equal(t, []legal.Acceptance{noIP, first, acceptance(u, "tos", "2.0.0", "second")}, acceptancesOf(t, r, u))
}

func testPolicyAcceptanceInsertRace(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	u := id.NewUserID()

	var wg sync.WaitGroup
	errs := make([]error, 16)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = c.PolicyAcceptance.InsertIfAbsent(ctx, []legal.Acceptance{acceptance(u, "tos", "1.0.0", "race")})
		}()
	}
	wg.Wait()
	for _, err := range errs {
		assert.NoError(t, err)
	}
	assert.Len(t, acceptancesOf(t, c.PolicyAcceptance, u), 1)
}

func testPolicyAcceptanceRemove(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	r := c.PolicyAcceptance
	u, other := id.NewUserID(), id.NewUserID()
	stored := acceptance(u, "tos", "1.0.0", "ua")
	require.NoError(t, r.InsertIfAbsent(ctx, []legal.Acceptance{stored, acceptance(other, "tos", "1.0.0", "ua")}))

	later := stored
	later.AcceptedAt = later.AcceptedAt.Add(time.Millisecond)
	require.NoError(t, r.Remove(ctx, []legal.Acceptance{later}))
	assert.Equal(t, []legal.Acceptance{stored}, acceptancesOf(t, r, u))

	require.NoError(t, r.Remove(ctx, []legal.Acceptance{stored, acceptance(u, "privacy", "1.0.0", "ua")}))
	require.NoError(t, r.Remove(ctx, nil))
	assert.Empty(t, acceptancesOf(t, r, u))
	assert.Len(t, acceptancesOf(t, r, other), 1)
}

func testPolicyAcceptanceRemoveByUser(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	r := c.PolicyAcceptance
	gone, kept := id.NewUserID(), id.NewUserID()

	require.NoError(t, r.InsertIfAbsent(ctx, []legal.Acceptance{
		acceptance(gone, "tos", "1.0.0", "ua"),
		acceptance(gone, "privacy", "1.0.0", "ua"),
		acceptance(kept, "tos", "1.0.0", "ua"),
	}))
	require.NoError(t, r.RemoveByUser(ctx, gone))
	require.NoError(t, r.RemoveByUser(ctx, gone))

	assert.Empty(t, acceptancesOf(t, r, gone))
	assert.Equal(t, []legal.Acceptance{acceptance(kept, "tos", "1.0.0", "ua")}, acceptancesOf(t, r, kept))
}

func testPolicyAcceptanceRemoveRollsBack(t *testing.T, nc Factory) {
	c, caps, done := nc(t)
	defer done()
	if !caps.RealTransactions {
		t.Skip("backend has no transactions")
	}
	ctx := context.Background()
	u := id.NewUserID()
	require.NoError(t, c.PolicyAcceptance.InsertIfAbsent(ctx, []legal.Acceptance{acceptance(u, "tos", "1.0.0", "ua")}))

	err := usecasex.DoTransaction(ctx, c.Transaction, 0, func(ctx context.Context) error {
		require.NoError(t, c.PolicyAcceptance.RemoveByUser(ctx, u))
		return assert.AnError
	})
	assert.ErrorIs(t, err, assert.AnError)
	assert.Len(t, acceptancesOf(t, c.PolicyAcceptance, u), 1)
}

var tosOne = []legal.RefInput{{Type: "tos", Version: "1.0.0"}}

func acceptFixture(t *testing.T, c *repo.Container) {
	t.Helper()
	now := time.Now()
	_, err := legal.Apply(context.Background(), c.PolicyVersion, legal.Manifest{Timestamp: now.Add(-time.Minute), Entries: []legal.Entry{{
		EffectiveAt: now.Add(-time.Hour),
		Ref:         legal.Ref{Type: "tos", Version: legal.MustParseVersion("1.0.0")},
		Required:    true,
		Title:       legal.LocalizedText{"en": "ToS"},
	}}}, func() time.Time { return now.Add(-time.Minute) })
	require.NoError(t, err)
}

func newPolicy(c *repo.Container) *interactor.Policy {
	return interactor.NewPolicy(c, legal.NewSecret("s"), interactor.NewPolicyCatalog(c.PolicyVersion, interactor.PolicyCatalogTTL))
}

func newUsers(c *repo.Container) interfaces.User {
	return interactor.NewUser(c, &gateway.Container{}, nil, "", "")
}

func seedAccepter(t *testing.T, c *repo.Container, n int) (id.UserID, *workspace.Operator) {
	t.Helper()
	ctx := context.Background()
	u := newUser(t, "racer", fmt.Sprintf("racer%d@example.com", n))
	require.NoError(t, c.User.Save(ctx, u))
	ws, err := workspace.New().ID(u.Workspace()).Name("racer").Personal(true).
		Members(map[id.UserID]workspace.Member{u.ID(): {Role: role.RoleOwner, InvitedBy: u.ID()}}).Build()
	require.NoError(t, err)
	require.NoError(t, c.Workspace.Save(ctx, ws))
	uid := u.ID()
	return uid, &workspace.Operator{OwningWorkspaces: id.WorkspaceIDList{ws.ID()}, User: &uid}
}

func testPolicyAcceptanceAcceptRacingDeleteMe(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	acceptFixture(t, c)
	policy, users := newPolicy(c), newUsers(c)

	orphans := 0
	const rounds = 300
	for i := range rounds {
		uid, op := seedAccepter(t, c, i)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _ = policy.Accept(ctx, tosOne, legal.Client{}, op)
		}()
		go func() {
			defer wg.Done()
			assert.NoError(t, users.DeleteMe(ctx, uid, op))
		}()
		wg.Wait()
		if len(acceptancesOf(t, c.PolicyAcceptance, uid)) > 0 {
			orphans++
		}
	}
	assert.Zero(t, orphans, "deleted users that still have acceptance records, of %d", rounds)
}

type hookedAcceptances struct {
	legal.AcceptanceRepo
	afterInsert func()
	beforeSweep func(call int)
	sweeps      *int
}

func (h hookedAcceptances) InsertIfAbsent(ctx context.Context, as []legal.Acceptance) error {
	err := h.AcceptanceRepo.InsertIfAbsent(ctx, as)
	if h.afterInsert != nil {
		h.afterInsert()
	}
	return err
}

func (h hookedAcceptances) RemoveByUser(ctx context.Context, u id.UserID) error {
	if h.beforeSweep == nil {
		return h.AcceptanceRepo.RemoveByUser(ctx, u)
	}
	*h.sweeps++
	call := *h.sweeps
	if call == 2 {
		h.beforeSweep(call)
	}
	err := h.AcceptanceRepo.RemoveByUser(ctx, u)
	if call == 1 {
		h.beforeSweep(call)
	}
	return err
}

func testPolicyAcceptanceAcceptCutOffAfterDeleteMe(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	acceptFixture(t, c)
	uid, op := seedAccepter(t, c, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, newUsers(c).DeleteMe(context.Background(), uid, op))

	hooked := *c
	hooked.PolicyAcceptance = hookedAcceptances{AcceptanceRepo: c.PolicyAcceptance, afterInsert: cancel}
	_, err := newPolicy(&hooked).Accept(ctx, tosOne, legal.Client{Origin: "o", UserAgent: "ua"}, op)

	assert.Error(t, err)
	assert.Empty(t, acceptancesOf(t, c.PolicyAcceptance, uid))
}

type failingUserRead struct{ user.Repo }

func (failingUserRead) FindByID(context.Context, id.UserID) (*user.User, error) {
	return nil, assert.AnError
}

func testPolicyAcceptanceAcceptReReadFails(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	acceptFixture(t, c)
	uid, op := seedAccepter(t, c, 0)
	earlier := acceptance(uid, "privacy", "1.0.0", "ua")
	require.NoError(t, c.PolicyAcceptance.InsertIfAbsent(context.Background(), []legal.Acceptance{earlier}))

	hooked := *c
	hooked.User = failingUserRead{c.User}
	_, err := newPolicy(&hooked).Accept(context.Background(), tosOne, legal.Client{UserAgent: "ua"}, op)

	assert.ErrorIs(t, err, assert.AnError)
	assert.Equal(t, []legal.Acceptance{earlier}, acceptancesOf(t, c.PolicyAcceptance, uid))
}

func testPolicyAcceptanceDeleteMeSweepCutOff(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	acceptFixture(t, c)
	uid, op := seedAccepter(t, c, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var acceptErr error
	sweeps := 0
	hooked := *c
	hooked.PolicyAcceptance = hookedAcceptances{AcceptanceRepo: c.PolicyAcceptance, sweeps: &sweeps, beforeSweep: func(call int) {
		if call == 1 {
			_, acceptErr = newPolicy(c).Accept(context.Background(), tosOne, legal.Client{UserAgent: "ua"}, op)
			return
		}
		cancel()
	}}
	err := newUsers(&hooked).DeleteMe(ctx, uid, op)

	require.NoError(t, acceptErr)
	assert.NoError(t, err)
	assert.Equal(t, 2, sweeps)
	_, err = c.User.FindByID(context.Background(), uid)
	assert.ErrorIs(t, err, rerror.ErrNotFound)
	assert.Empty(t, acceptancesOf(t, c.PolicyAcceptance, uid))
}
