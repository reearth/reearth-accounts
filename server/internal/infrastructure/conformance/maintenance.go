package conformance

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/usecase/maintenance"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/legal"
	"github.com/reearth/reearth-accounts/server/pkg/outbox"
	"github.com/reearth/reearthx/rerror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runMaintenance(t *testing.T, nc Factory) {
	t.Run("Outbox_DeleteMeRemovesRecords", func(t *testing.T) { testOutboxDeleteMe(t, nc) })
	t.Run("Maintenance_OrphanAcceptancesRetention", func(t *testing.T) { testOrphanAcceptancesRetention(t, nc) })
}

func testOutboxDeleteMe(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	uid, op := seedAccepter(t, c, 0)
	other := id.NewUserID()
	require.NoError(t, c.Outbox.Enqueue(ctx, []outbox.Record{
		outboxRecord(t, "mine", outbox.ToUser(uid), timeFixed()),
		outboxRecord(t, "theirs", outbox.ToUser(other), timeFixed()),
	}))
	require.NoError(t, newUsers(c).DeleteMe(ctx, uid, op))
	_, err := c.Outbox.FindByKey(ctx, "mine")
	assert.ErrorIs(t, err, rerror.ErrNotFound)
	findRecord(t, c, "theirs")
}

func testOrphanAcceptancesRetention(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	kept, _ := seedAccepter(t, c, 0)
	gone := []id.UserID{id.NewUserID(), id.NewUserID(), id.NewUserID()}
	var as []legal.Acceptance
	for _, u := range append([]id.UserID{kept}, gone...) {
		as = append(as, acceptance(u, "tos", "1.0.0", "ua"), acceptance(u, "privacy", "1.0.0", "ua"))
	}
	require.NoError(t, c.PolicyAcceptance.InsertIfAbsent(ctx, as))

	first, err := c.PolicyAcceptance.FindUsersAfter(ctx, nil, 2)
	require.NoError(t, err)
	require.Len(t, first, 2)
	assert.Less(t, first[0].String(), first[1].String())
	rest, err := c.PolicyAcceptance.FindUsersAfter(ctx, &first[1], 10)
	require.NoError(t, err)
	assert.Len(t, rest, 2)
	assert.Less(t, first[1].String(), rest[0].String())

	h := maintenance.OrphanAcceptances(c.PolicyAcceptance, c.User, 2)
	total := 0
	for range 4 {
		n, err := h.Run(ctx, timeFixed())
		require.NoError(t, err)
		total += n
	}
	assert.Equal(t, len(gone), total)
	for _, u := range gone {
		assert.Empty(t, acceptancesOf(t, c.PolicyAcceptance, u))
	}
	assert.Len(t, acceptancesOf(t, c.PolicyAcceptance, kept), 2)
}
