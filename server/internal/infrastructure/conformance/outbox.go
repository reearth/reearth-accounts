package conformance

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/mail"
	"github.com/reearth/reearth-accounts/server/pkg/outbox"
	"github.com/reearth/reearthx/rerror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testKind mail.Kind = "outbox_conformance"

func runOutbox(t *testing.T, nc Factory) {
	t.Run("Outbox_EnqueueIfAbsent", func(t *testing.T) { testOutboxEnqueueIfAbsent(t, nc) })
	t.Run("Outbox_ClaimDue", func(t *testing.T) { testOutboxClaimDue(t, nc) })
	t.Run("Outbox_ClaimExclusive", func(t *testing.T) { testOutboxClaimExclusive(t, nc) })
	t.Run("Outbox_CompleteNeedsLiveClaim", func(t *testing.T) { testOutboxCompleteNeedsLiveClaim(t, nc) })
	t.Run("Outbox_CompleteRetry", func(t *testing.T) { testOutboxCompleteRetry(t, nc) })
	t.Run("Outbox_Sweep", func(t *testing.T) { testOutboxSweep(t, nc) })
	t.Run("Outbox_RemoveByUser", func(t *testing.T) { testOutboxRemoveByUser(t, nc) })
}

func outboxRecord(t *testing.T, key string, to outbox.Recipient, now time.Time) outbox.Record {
	t.Helper()
	r, err := outbox.New(outbox.Key(key), testKind, outbox.Subject{"ref": key}, to, now)
	require.NoError(t, err)
	return r
}

func findRecord(t *testing.T, c *repo.Container, key string) *outbox.Record {
	t.Helper()
	r, err := c.Outbox.FindByKey(context.Background(), outbox.Key(key))
	require.NoError(t, err)
	return r
}

func claim(token string, until time.Time) outbox.Claim {
	return outbox.Claim{Token: token, Until: until}
}

func testOutboxEnqueueIfAbsent(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	now := timeFixed()
	u := id.NewUserID()
	first := outboxRecord(t, "k1", outbox.ToUser(u), now)
	require.NoError(t, c.Outbox.Enqueue(ctx, []outbox.Record{first, outboxRecord(t, "k2", outbox.ToAddress("Guest@Example.com"), now)}))

	again := outboxRecord(t, "k1", outbox.ToUser(id.NewUserID()), now.Add(time.Hour))
	again.Subject = outbox.Subject{"ref": "changed"}
	require.NoError(t, c.Outbox.Enqueue(ctx, []outbox.Record{again, outboxRecord(t, "k3", outbox.ToUser(u), now)}))

	got := findRecord(t, c, "k1")
	assert.Equal(t, first, *got)
	addr, ok := findRecord(t, c, "k2").Recipient.Address()
	assert.True(t, ok)
	assert.Equal(t, "guest@example.com", addr)
	assert.Equal(t, outbox.StatePending, findRecord(t, c, "k3").State)

	_, err := c.Outbox.FindByKey(ctx, "missing")
	assert.ErrorIs(t, err, rerror.ErrNotFound)
	require.NoError(t, c.Outbox.Enqueue(ctx, nil))
}

func testOutboxClaimDue(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	now := timeFixed()
	later := outboxRecord(t, "later", outbox.ToUser(id.NewUserID()), now)
	later.NextAttemptAt = now.Add(-time.Minute)
	earlier := outboxRecord(t, "earlier", outbox.ToUser(id.NewUserID()), now)
	earlier.NextAttemptAt = now.Add(-time.Hour)
	future := outboxRecord(t, "future", outbox.ToUser(id.NewUserID()), now)
	future.NextAttemptAt = now.Add(time.Minute)
	require.NoError(t, c.Outbox.Enqueue(ctx, []outbox.Record{later, earlier, future}))

	got, err := c.Outbox.ClaimDue(ctx, now, claim("t1", now.Add(outbox.ClaimLease)))
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, outbox.Key("earlier"), got.Key)
	assert.Equal(t, outbox.StateSending, got.State)
	assert.Equal(t, 1, got.Attempts)
	assert.Equal(t, &outbox.Claim{Token: "t1", Until: now.Add(outbox.ClaimLease)}, got.Claim)
	assert.Equal(t, outbox.Subject{"ref": "earlier"}, got.Subject)

	got, err = c.Outbox.ClaimDue(ctx, now, claim("t2", now.Add(outbox.ClaimLease)))
	require.NoError(t, err)
	assert.Equal(t, outbox.Key("later"), got.Key)

	got, err = c.Outbox.ClaimDue(ctx, now, claim("t3", now.Add(outbox.ClaimLease)))
	require.NoError(t, err)
	assert.Nil(t, got, "a record due in the future is not claimed")
}

func testOutboxClaimExclusive(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	now := timeFixed()
	const records, claimers = 40, 16
	rs := make([]outbox.Record, records)
	for i := range rs {
		rs[i] = outboxRecord(t, fmt.Sprintf("r%02d", i), outbox.ToUser(id.NewUserID()), now)
	}
	require.NoError(t, c.Outbox.Enqueue(ctx, rs))

	var (
		mu      sync.Mutex
		claimed = map[outbox.Key]string{}
		wg      sync.WaitGroup
		start   = make(chan struct{})
	)
	for w := range claimers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for n := 0; ; n++ {
				token := fmt.Sprintf("w%d-%d", w, n)
				r, err := c.Outbox.ClaimDue(ctx, now, claim(token, now.Add(outbox.ClaimLease)))
				if !assert.NoError(t, err) || r == nil {
					return
				}
				mu.Lock()
				prev, dup := claimed[r.Key]
				claimed[r.Key] = token
				mu.Unlock()
				assert.False(t, dup, "%s claimed by %s and %s", r.Key, prev, token)
			}
		}()
	}
	close(start)
	wg.Wait()
	assert.Len(t, claimed, records)
	for k, token := range claimed {
		r := findRecord(t, c, string(k))
		assert.Equal(t, token, r.Claim.Token)
		assert.Equal(t, 1, r.Attempts)
	}
}

func testOutboxCompleteNeedsLiveClaim(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	now := timeFixed()
	require.NoError(t, c.Outbox.Enqueue(ctx, []outbox.Record{outboxRecord(t, "k", outbox.ToUser(id.NewUserID()), now)}))
	until := now.Add(outbox.ClaimLease)
	_, err := c.Outbox.ClaimDue(ctx, now, claim("tok", until))
	require.NoError(t, err)
	sent := outbox.Transition{To: outbox.StateSent}

	ok, err := c.Outbox.Complete(ctx, "k", "other", sent, now)
	require.NoError(t, err)
	assert.False(t, ok, "wrong token")

	ok, err = c.Outbox.Complete(ctx, "k", "tok", sent, until)
	require.NoError(t, err)
	assert.False(t, ok, "claim expired at its until")
	assert.Equal(t, outbox.StateSending, findRecord(t, c, "k").State)

	ok, err = c.Outbox.Complete(ctx, "k", "tok", sent, until.Add(-time.Millisecond))
	require.NoError(t, err)
	assert.True(t, ok)
	got := findRecord(t, c, "k")
	assert.Equal(t, outbox.StateSent, got.State)
	assert.Nil(t, got.Claim)
	require.NotNil(t, got.EndedAt)
	assert.True(t, got.EndedAt.Equal(until.Add(-time.Millisecond)))

	ok, err = c.Outbox.Complete(ctx, "k", "tok", outbox.Transition{To: outbox.StateFailed}, now)
	require.NoError(t, err)
	assert.False(t, ok, "a result is written once")

	ok, err = c.Outbox.Complete(ctx, "missing", "tok", sent, now)
	require.NoError(t, err)
	assert.False(t, ok)
}

func testOutboxCompleteRetry(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	now := timeFixed()
	require.NoError(t, c.Outbox.Enqueue(ctx, []outbox.Record{outboxRecord(t, "k", outbox.ToUser(id.NewUserID()), now)}))
	_, err := c.Outbox.ClaimDue(ctx, now, claim("tok", now.Add(outbox.ClaimLease)))
	require.NoError(t, err)
	next := now.Add(time.Minute)

	ok, err := c.Outbox.Complete(ctx, "k", "tok", outbox.Transition{To: outbox.StatePending, NextAttemptAt: next}, now)
	require.NoError(t, err)
	require.True(t, ok)
	got := findRecord(t, c, "k")
	assert.Equal(t, outbox.StatePending, got.State)
	assert.True(t, got.NextAttemptAt.Equal(next))
	assert.Nil(t, got.Claim)
	assert.Nil(t, got.EndedAt)

	r, err := c.Outbox.ClaimDue(ctx, now, claim("tok2", now.Add(outbox.ClaimLease)))
	require.NoError(t, err)
	assert.Nil(t, r, "not due before its next attempt")
	r, err = c.Outbox.ClaimDue(ctx, next, claim("tok2", next.Add(outbox.ClaimLease)))
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, 2, r.Attempts)

	ok, err = c.Outbox.Complete(ctx, "k", "tok2", outbox.Transition{To: outbox.StateFailed, Reason: outbox.ReasonMaxAttempts}, next)
	require.NoError(t, err)
	require.True(t, ok)
	got = findRecord(t, c, "k")
	assert.Equal(t, outbox.StateFailed, got.State)
	assert.Equal(t, outbox.ReasonMaxAttempts, got.Reason)
}

func testOutboxSweep(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	now := timeFixed()
	ttl := 720 * time.Hour
	t0 := now.Add(-3 * ttl)
	record := func(key string, created, next time.Time) outbox.Record {
		r := outboxRecord(t, key, outbox.ToUser(id.NewUserID()), created)
		r.NextAttemptAt = next
		return r
	}
	require.NoError(t, c.Outbox.Enqueue(ctx, []outbox.Record{
		record("old-sent", t0, t0.Add(-3*time.Hour)),
		record("new-sent", t0, t0.Add(-2*time.Hour)),
		record("claimed", t0, t0.Add(-time.Hour)),
		record("stale", now.Add(-ttl), now.Add(time.Hour)),
		record("fresh", now.Add(-ttl+time.Second), now.Add(time.Hour)),
	}))
	for _, want := range []struct {
		key   string
		until time.Time
	}{{"old-sent", t0.Add(time.Hour)}, {"new-sent", now}, {"claimed", t0.Add(time.Minute)}} {
		r, err := c.Outbox.ClaimDue(ctx, t0, claim(want.key, want.until))
		require.NoError(t, err)
		require.Equal(t, outbox.Key(want.key), r.Key)
	}
	sent := outbox.Transition{To: outbox.StateSent}
	for key, at := range map[string]time.Time{"old-sent": t0, "new-sent": now.Add(-time.Minute)} {
		ok, err := c.Outbox.Complete(ctx, outbox.Key(key), key, sent, at)
		require.NoError(t, err)
		require.True(t, ok, key)
	}

	res, err := c.Outbox.Sweep(ctx, now, ttl)
	require.NoError(t, err)
	assert.Equal(t, outbox.SweepResult{Expired: 1, TimedOut: 1, Deleted: 1}, res)

	claimed := findRecord(t, c, "claimed")
	assert.Equal(t, outbox.StateUnknown, claimed.State)
	assert.Equal(t, outbox.ReasonClaimExpired, claimed.Reason)
	assert.Nil(t, claimed.Claim)
	stale := findRecord(t, c, "stale")
	assert.Equal(t, outbox.StateFailed, stale.State)
	assert.Equal(t, outbox.ReasonTTL, stale.Reason)
	require.NotNil(t, stale.EndedAt)
	assert.True(t, stale.EndedAt.Equal(now))
	assert.Equal(t, outbox.StatePending, findRecord(t, c, "fresh").State)
	assert.Equal(t, outbox.StateSent, findRecord(t, c, "new-sent").State)
	_, err = c.Outbox.FindByKey(ctx, "old-sent")
	assert.ErrorIs(t, err, rerror.ErrNotFound)

	res, err = c.Outbox.Sweep(ctx, now, ttl)
	require.NoError(t, err)
	assert.Equal(t, outbox.SweepResult{}, res, "a second sweep finds nothing")

	res, err = c.Outbox.Sweep(ctx, now.Add(ttl), ttl)
	require.NoError(t, err)
	assert.Equal(t, outbox.SweepResult{TimedOut: 1, Deleted: 3}, res, "fresh times out; claimed, stale and new-sent ended a TTL ago")
}

func testOutboxRemoveByUser(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	now := timeFixed()
	u, other := id.NewUserID(), id.NewUserID()
	require.NoError(t, c.Outbox.Enqueue(ctx, []outbox.Record{
		outboxRecord(t, "a", outbox.ToUser(u), now),
		outboxRecord(t, "b", outbox.ToUser(u), now),
		outboxRecord(t, "c", outbox.ToUser(other), now),
		outboxRecord(t, "d", outbox.ToAddress("guest@example.com"), now),
	}))
	require.NoError(t, c.Outbox.RemoveByUser(ctx, u))
	for _, k := range []outbox.Key{"a", "b"} {
		_, err := c.Outbox.FindByKey(ctx, k)
		assert.ErrorIs(t, err, rerror.ErrNotFound)
	}
	findRecord(t, c, "c")
	findRecord(t, c, "d")
}
