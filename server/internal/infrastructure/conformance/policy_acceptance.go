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
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/legal"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/usecasex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runPolicyAcceptance(t *testing.T, nc Factory) {
	t.Run("PolicyAcceptance_InsertIfAbsent", func(t *testing.T) { testPolicyAcceptanceInsertIfAbsent(t, nc) })
	t.Run("PolicyAcceptance_InsertRace", func(t *testing.T) { testPolicyAcceptanceInsertRace(t, nc) })
	t.Run("PolicyAcceptance_RemoveByUser", func(t *testing.T) { testPolicyAcceptanceRemoveByUser(t, nc) })
	t.Run("PolicyAcceptance_RemoveRollsBack", func(t *testing.T) { testPolicyAcceptanceRemoveRollsBack(t, nc) })
	t.Run("PolicyAcceptance_AcceptRacingDeleteMe", func(t *testing.T) { testPolicyAcceptanceAcceptRacingDeleteMe(t, nc) })
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

func testPolicyAcceptanceAcceptRacingDeleteMe(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	now := time.Now()
	_, err := legal.Apply(ctx, c.PolicyVersion, legal.Manifest{Timestamp: now.Add(-time.Minute), Entries: []legal.Entry{{
		EffectiveAt: now.Add(-time.Hour),
		Ref:         legal.Ref{Type: "tos", Version: legal.MustParseVersion("1.0.0")},
		Required:    true,
		Title:       legal.LocalizedText{"en": "ToS"},
	}}}, func() time.Time { return now.Add(-time.Minute) })
	require.NoError(t, err)
	policy := interactor.NewPolicy(c, legal.NewSecret("s"), interactor.NewPolicyCatalog(c.PolicyVersion, interactor.PolicyCatalogTTL))
	users := interactor.NewUser(c, &gateway.Container{}, nil, "", "")

	orphans := 0
	const rounds = 300
	for i := range rounds {
		u := newUser(t, "racer", fmt.Sprintf("racer%d@example.com", i))
		require.NoError(t, c.User.Save(ctx, u))
		ws, err := workspace.New().ID(u.Workspace()).Name("racer").Personal(true).
			Members(map[id.UserID]workspace.Member{u.ID(): {Role: role.RoleOwner, InvitedBy: u.ID()}}).Build()
		require.NoError(t, err)
		require.NoError(t, c.Workspace.Save(ctx, ws))
		uid := u.ID()
		op := &workspace.Operator{OwningWorkspaces: id.WorkspaceIDList{ws.ID()}, User: &uid}

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _ = policy.Accept(ctx, []legal.RefInput{{Type: "tos", Version: "1.0.0"}}, legal.Client{}, op)
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
