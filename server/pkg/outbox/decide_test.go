package outbox

import (
	"testing"
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	t0     = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	ttl    = 720 * time.Hour
	policy = RetryPolicy{BaseDelay: time.Minute, MaxAttempts: 5, MaxDelay: time.Hour}
)

func noJitter(d time.Duration) time.Duration { return d }

func queued(t *testing.T) Record {
	t.Helper()
	r, err := New("k", "test_kind", Subject{"ref": "1"}, ToUser(id.NewUserID()), t0)
	require.NoError(t, err)
	return r
}

func sending(t *testing.T, attempts int) Record {
	t.Helper()
	r := queued(t)
	for range attempts {
		r = r.Claimed(Claim{Token: "tok", Until: t0.Add(ClaimLease)}, t0)
		if r.Attempts < attempts {
			r = r.Applied(Transition{To: StatePending, NextAttemptAt: t0}, t0)
		}
	}
	return r
}

func TestNew(t *testing.T) {
	r := queued(t)
	assert.Equal(t, StatePending, r.State)
	assert.Equal(t, 0, r.Attempts)
	assert.True(t, r.Due(t0), "queued -> pending, due at once")

	_, err := New("", "k", nil, ToUser(id.NewUserID()), t0)
	assert.ErrorIs(t, err, ErrEmptyKey)
	_, err = New("k", "", nil, ToUser(id.NewUserID()), t0)
	assert.ErrorIs(t, err, ErrEmptyKind)
	_, err = New("k", "k", nil, Recipient{}, t0)
	assert.ErrorIs(t, err, ErrInvalidRecipient)
	_, err = New("k", "k", nil, ToAddress("  "), t0)
	assert.ErrorIs(t, err, ErrInvalidRecipient)
}

func TestRecipient(t *testing.T) {
	u := id.NewUserID()
	got, ok := ToUser(u).User()
	assert.True(t, ok)
	assert.Equal(t, u, got)
	_, ok = ToUser(u).Address()
	assert.False(t, ok)

	a, ok := ToAddress(" Guest@Example.COM ").Address()
	assert.True(t, ok)
	assert.Equal(t, "guest@example.com", a)
	_, ok = ToAddress("guest@example.com").User()
	assert.False(t, ok)
}

func TestPendingToSending(t *testing.T) {
	r := queued(t)
	r.NextAttemptAt = t0.Add(time.Minute)
	assert.False(t, r.Due(t0), "not due before its next attempt")
	assert.True(t, r.Due(t0.Add(time.Minute)))

	c := Claim{Token: "tok", Until: t0.Add(ClaimLease)}
	got := r.Claimed(c, t0)
	assert.Equal(t, StateSending, got.State)
	assert.Equal(t, 1, got.Attempts)
	assert.Equal(t, &c, got.Claim)
	assert.False(t, got.Due(t0.Add(time.Hour)), "a sending record is never claimed again")
}

func TestPendingToFailedAfterTTL(t *testing.T) {
	r := queued(t)
	assert.False(t, r.Stale(t0.Add(ttl-time.Second), ttl))
	assert.True(t, r.Stale(t0.Add(ttl), ttl))

	got := r.TimedOut(t0.Add(ttl))
	assert.Equal(t, StateFailed, got.State)
	assert.Equal(t, ReasonTTL, got.Reason)
	assert.Equal(t, t0.Add(ttl), *got.EndedAt)

	assert.False(t, sending(t, 1).Stale(t0.Add(2*ttl), ttl), "only pending records time out")
}

func TestSendingToSent(t *testing.T) {
	r := sending(t, 1)
	tr := Decide(r, Delivered{}, policy, t0, noJitter)
	assert.Equal(t, Transition{To: StateSent}, tr)

	got := r.Applied(tr, t0.Add(time.Second))
	assert.Equal(t, StateSent, got.State)
	assert.Nil(t, got.Claim)
	assert.Equal(t, t0.Add(time.Second), *got.EndedAt)
}

func TestSendingToPendingOnTransientError(t *testing.T) {
	for attempts, wait := range map[int]time.Duration{1: time.Minute, 2: 2 * time.Minute, 3: 4 * time.Minute, 4: 8 * time.Minute} {
		tr := Decide(sending(t, attempts), SendFailed{}, policy, t0, noJitter)
		assert.Equal(t, Transition{To: StatePending, NextAttemptAt: t0.Add(wait)}, tr, "attempt %d", attempts)
	}

	r := sending(t, 2)
	got := r.Applied(Decide(r, SendFailed{}, policy, t0, noJitter), t0)
	assert.Equal(t, StatePending, got.State)
	assert.Nil(t, got.Claim)
	assert.Nil(t, got.EndedAt)
	assert.True(t, got.Due(t0.Add(2*time.Minute)))
	assert.Equal(t, 3, got.Claimed(Claim{Token: "t2", Until: t0.Add(ClaimLease)}, t0).Attempts)
}

func TestBackoffIsCappedAndJittered(t *testing.T) {
	p := RetryPolicy{BaseDelay: time.Minute, MaxAttempts: 20, MaxDelay: time.Hour}
	tr := Decide(sending(t, 15), SendFailed{}, p, t0, noJitter)
	assert.Equal(t, t0.Add(time.Hour), tr.NextAttemptAt)

	half := func(d time.Duration) time.Duration { return d / 2 }
	tr = Decide(sending(t, 3), SendFailed{}, p, t0, half)
	assert.Equal(t, t0.Add(2*time.Minute), tr.NextAttemptAt)
}

func TestSendingToFailed(t *testing.T) {
	assert.Equal(t, Transition{To: StateFailed, Reason: ReasonPermanent},
		Decide(sending(t, 1), SendFailed{Permanent: true}, policy, t0, noJitter), "permanent error")
	assert.Equal(t, Transition{To: StateFailed, Reason: ReasonMaxAttempts},
		Decide(sending(t, 5), SendFailed{}, policy, t0, noJitter), "no attempts left")
	assert.Equal(t, Transition{To: StateFailed, Reason: "user_gone"},
		Decide(sending(t, 1), NothingToSend{Reason: "user_gone"}, policy, t0, noJitter), "nothing to send")

	got := sending(t, 1).Applied(Transition{To: StateFailed, Reason: ReasonPermanent}, t0)
	assert.Equal(t, StateFailed, got.State)
	assert.Equal(t, ReasonPermanent, got.Reason)
	assert.NotNil(t, got.EndedAt)
}

func TestSendingToUnknownWhenTheClaimExpires(t *testing.T) {
	r := sending(t, 1)
	until := r.Claim.Until
	assert.True(t, r.Holds("tok", until.Add(-time.Nanosecond)))
	assert.False(t, r.Holds("tok", until), "a claim holds only before its until")
	assert.False(t, r.Holds("other", t0))
	assert.False(t, r.ClaimExpired(until.Add(-time.Nanosecond)))
	assert.True(t, r.ClaimExpired(until))

	got := r.Expired(until)
	assert.Equal(t, StateUnknown, got.State)
	assert.Equal(t, ReasonClaimExpired, got.Reason)
	assert.Nil(t, got.Claim)
	assert.False(t, got.Due(until.Add(ttl)), "an unknown record is never sent again")
	assert.False(t, got.Holds("tok", t0), "a late result is not written")
}

func TestEndedRecordsAreDeletedAfterTheTTL(t *testing.T) {
	end := t0.Add(time.Hour)
	ended := map[State]Record{
		StateSent:    sending(t, 1).Applied(Transition{To: StateSent}, end),
		StateFailed:  sending(t, 1).Applied(Transition{To: StateFailed, Reason: ReasonPermanent}, end),
		StateUnknown: sending(t, 1).Expired(end),
	}
	for s, r := range ended {
		assert.True(t, s.Ended())
		assert.False(t, r.Retired(end.Add(ttl-time.Second), ttl), s)
		assert.True(t, r.Retired(end.Add(ttl), ttl), s)
	}
	assert.False(t, queued(t).Retired(t0.Add(10*ttl), ttl))
	assert.False(t, sending(t, 1).Retired(t0.Add(10*ttl), ttl))
	assert.False(t, StatePending.Ended())
	assert.False(t, StateSending.Ended())
}
