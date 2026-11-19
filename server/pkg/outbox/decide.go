package outbox

import "time"

type Outcome interface{ outcome() }

type Delivered struct{}

type SendFailed struct{ Permanent bool }

// NothingToSend means the mail no longer applies, for example because its
// user is gone.
type NothingToSend struct{ Reason Reason }

func (Delivered) outcome()     {}
func (SendFailed) outcome()    {}
func (NothingToSend) outcome() {}

type RetryPolicy struct {
	BaseDelay   time.Duration
	MaxAttempts int
	MaxDelay    time.Duration
}

var DefaultRetryPolicy = RetryPolicy{BaseDelay: time.Minute, MaxAttempts: 5, MaxDelay: time.Hour}

type Transition struct {
	NextAttemptAt time.Time
	Reason        Reason
	To            State
}

// Decide maps a claimed record and its outcome to the next state. A transient
// failure waits BaseDelay doubled per attempt, capped at MaxDelay, then
// passed through jitter.
func Decide(r Record, o Outcome, p RetryPolicy, now time.Time, jitter func(time.Duration) time.Duration) Transition {
	switch o := o.(type) {
	case Delivered:
		return Transition{To: StateSent}
	case NothingToSend:
		return Transition{To: StateFailed, Reason: o.Reason}
	case SendFailed:
		if o.Permanent {
			return Transition{To: StateFailed, Reason: ReasonPermanent}
		}
		if r.Attempts >= p.MaxAttempts {
			return Transition{To: StateFailed, Reason: ReasonMaxAttempts}
		}
		return Transition{To: StatePending, NextAttemptAt: now.Add(jitter(backoff(r.Attempts, p)))}
	}
	panic("outbox: unknown outcome")
}

func backoff(attempts int, p RetryPolicy) time.Duration {
	d := p.BaseDelay
	for i := 1; i < attempts && d < p.MaxDelay; i++ {
		d *= 2
	}
	return min(d, p.MaxDelay)
}

func (r Record) Due(now time.Time) bool {
	return r.State == StatePending && !r.NextAttemptAt.After(now)
}

func (r Record) Claimed(c Claim, now time.Time) Record {
	r.State = StateSending
	r.Attempts++
	r.Claim = &c
	r.UpdatedAt = now
	return r
}

func (r Record) Holds(token string, now time.Time) bool {
	return r.State == StateSending && r.Claim != nil && r.Claim.Token == token && r.Claim.Until.After(now)
}

func (r Record) Applied(t Transition, now time.Time) Record {
	r.State = t.To
	r.Reason = t.Reason
	r.Claim = nil
	r.UpdatedAt = now
	if t.To == StatePending {
		r.NextAttemptAt = t.NextAttemptAt
	}
	return r.ended(now)
}

// ClaimExpired reports whether r was claimed by a pass that never wrote its
// result. Such a record may have been sent, so it is never sent again.
func (r Record) ClaimExpired(now time.Time) bool {
	return r.State == StateSending && r.Claim != nil && !r.Claim.Until.After(now)
}

func (r Record) Stale(now time.Time, ttl time.Duration) bool {
	return r.State == StatePending && !r.CreatedAt.After(now.Add(-ttl))
}

func (r Record) Retired(now time.Time, ttl time.Duration) bool {
	return r.State.Ended() && r.EndedAt != nil && !r.EndedAt.After(now.Add(-ttl))
}

func (r Record) Expired(now time.Time) Record {
	r.State = StateUnknown
	r.Reason = ReasonClaimExpired
	r.Claim = nil
	r.UpdatedAt = now
	return r.ended(now)
}

func (r Record) TimedOut(now time.Time) Record {
	r.State = StateFailed
	r.Reason = ReasonTTL
	r.UpdatedAt = now
	return r.ended(now)
}

func (r Record) ended(now time.Time) Record {
	if r.State.Ended() {
		r.EndedAt = &now
	}
	return r
}

type SweepResult struct {
	Deleted  int
	Expired  int
	TimedOut int
}
