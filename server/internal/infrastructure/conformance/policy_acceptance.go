package conformance

import (
	"context"
	"net/netip"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/legal"
	"github.com/reearth/reearthx/usecasex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runPolicyAcceptance(t *testing.T, nc Factory) {
	t.Run("PolicyAcceptance_InsertIfAbsent", func(t *testing.T) { testPolicyAcceptanceInsertIfAbsent(t, nc) })
	t.Run("PolicyAcceptance_InsertRace", func(t *testing.T) { testPolicyAcceptanceInsertRace(t, nc) })
	t.Run("PolicyAcceptance_RemoveByUser", func(t *testing.T) { testPolicyAcceptanceRemoveByUser(t, nc) })
	t.Run("PolicyAcceptance_RemoveRollsBack", func(t *testing.T) { testPolicyAcceptanceRemoveRollsBack(t, nc) })
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
	for i := range as {
		as[i].AcceptedAt = as[i].AcceptedAt.UTC()
	}
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
