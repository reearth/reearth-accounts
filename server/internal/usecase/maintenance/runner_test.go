package maintenance

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/mail"
	"github.com/reearth/reearth-accounts/server/pkg/outbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

const testKind mail.Kind = "runner_test"

var t0 = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

type testResolver struct{ nothing outbox.Reason }

func (r testResolver) Resolve(_ context.Context, rec outbox.Record) (*mail.Envelope, outbox.Reason, error) {
	if r.nothing != "" {
		return nil, r.nothing, nil
	}
	u, _ := rec.Recipient.User()
	msg, err := mail.Render(language.English, mail.VerificationData{UserName: "u", ActionURL: "https://example.com/" + rec.Subject["ref"]})
	if err != nil {
		return nil, "", err
	}
	return &mail.Envelope{To: mail.ToUser(u, "u@example.com", "u"), Message: msg}, "", nil
}

type fakeMail struct {
	blockOnce atomic.Bool
	err       error
	got       []mail.Envelope
	lock      sync.Mutex
	noSender  bool
	onSend    func()
	release   chan struct{}
	sent      chan mail.Envelope
}

func (m *fakeMail) Deliverable() bool { return !m.noSender }

func (m *fakeMail) Send(ctx context.Context, e mail.Envelope) error {
	if m.blockOnce.CompareAndSwap(true, false) {
		<-ctx.Done()
		return ctx.Err()
	}
	if m.onSend != nil {
		m.onSend()
	}
	if m.sent != nil {
		m.sent <- e
	}
	if m.release != nil {
		<-m.release
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	m.got = append(m.got, e)
	return m.err
}

func (m *fakeMail) count() int {
	m.lock.Lock()
	defer m.lock.Unlock()
	return len(m.got)
}

// ctxOutbox fails on a done context, as the MongoDB and PostgreSQL repos do.
type ctxOutbox struct{ outbox.Repo }

func (o ctxOutbox) ClaimDue(ctx context.Context, now time.Time, c outbox.Claim) (*outbox.Record, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return o.Repo.ClaimDue(ctx, now, c)
}

func (o ctxOutbox) Complete(ctx context.Context, k outbox.Key, token string, t outbox.Transition, now time.Time) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	return o.Repo.Complete(ctx, k, token, t, now)
}

type fixture struct {
	clock  time.Time
	mail   *fakeMail
	outbox outbox.Repo
	runner *Runner
}

func newFixture(t *testing.T, m *fakeMail, cfg Config, resolvers map[mail.Kind]Resolver, hooks ...Retention) *fixture {
	t.Helper()
	f := &fixture{clock: t0, mail: m, outbox: ctxOutbox{memory.NewOutbox()}}
	f.runner = NewRunner(cfg, f.outbox, m, resolvers, nil, hooks...)
	f.runner.now = func() time.Time { return f.clock }
	f.runner.jitter = func(d time.Duration) time.Duration { return d }
	return f
}

func defaultConfig() Config {
	return Config{Lease: outbox.ClaimLease, Retry: outbox.DefaultRetryPolicy, SendsPerSecond: 1000, TTL: 720 * time.Hour}
}

func sendable() map[mail.Kind]Resolver { return map[mail.Kind]Resolver{testKind: testResolver{}} }

func (f *fixture) enqueue(t *testing.T, kind mail.Kind, n int) []outbox.Key {
	t.Helper()
	keys := make([]outbox.Key, n)
	rs := make([]outbox.Record, n)
	for i := range rs {
		keys[i] = outbox.Key(fmt.Sprintf("test:%02d", i))
		r, err := outbox.New(keys[i], kind, outbox.Subject{"ref": string(keys[i])}, outbox.ToUser(id.NewUserID()), f.clock.Add(time.Duration(i-n)*time.Millisecond))
		require.NoError(t, err)
		rs[i] = r
	}
	require.NoError(t, f.outbox.Enqueue(context.Background(), rs))
	return keys
}

func (f *fixture) record(t *testing.T, k outbox.Key) outbox.Record {
	t.Helper()
	r, err := f.outbox.FindByKey(context.Background(), k)
	require.NoError(t, err)
	if r.Reason != "" {
		assert.Contains(t, outbox.Reasons, r.Reason, "a stored reason is one of outbox.Reasons")
	}
	return *r
}

func TestRunPass_SendsQueuedMail(t *testing.T) {
	f := newFixture(t, &fakeMail{}, defaultConfig(), sendable())
	keys := f.enqueue(t, testKind, 3)

	rep, err := f.runner.RunPass(context.Background())
	require.NoError(t, err)
	assert.True(t, rep.Ran)
	assert.Equal(t, 3, rep.Sent)
	for _, k := range keys {
		r := f.record(t, k)
		assert.Equal(t, outbox.StateSent, r.State)
		assert.Equal(t, 1, r.Attempts)
	}
	require.Equal(t, 3, f.mail.count())
	assert.Contains(t, f.mail.got[0].Message.Text(), "https://example.com/test:00", "the resolver builds the mail at send time")
}

func TestRunPass_NoProviderLeavesMailQueued(t *testing.T) {
	f := newFixture(t, &fakeMail{noSender: true}, defaultConfig(), sendable())
	keys := f.enqueue(t, testKind, 2)

	rep, err := f.runner.RunPass(context.Background())
	require.NoError(t, err)
	assert.True(t, rep.Ran)
	assert.Zero(t, rep.Sent)
	assert.Zero(t, f.mail.count())
	for _, k := range keys {
		assert.Equal(t, outbox.StatePending, f.record(t, k).State)
		assert.Zero(t, f.record(t, k).Attempts)
	}

	f.clock = t0.Add(720 * time.Hour)
	rep, err = f.runner.RunPass(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, rep.Sweep.TimedOut)
	assert.Equal(t, outbox.StateFailed, f.record(t, keys[0]).State)
	assert.Equal(t, outbox.ReasonTTL, f.record(t, keys[0]).Reason)
}

func TestRunPass_NothingToSendFails(t *testing.T) {
	f := newFixture(t, &fakeMail{}, defaultConfig(), map[mail.Kind]Resolver{testKind: testResolver{nothing: outbox.ReasonUserGone}})
	keys := f.enqueue(t, testKind, 1)

	rep, err := f.runner.RunPass(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, rep.Failed)
	assert.Zero(t, f.mail.count())
	r := f.record(t, keys[0])
	assert.Equal(t, outbox.StateFailed, r.State)
	assert.Equal(t, outbox.ReasonUserGone, r.Reason)
}

func TestRunPass_UnknownKindFails(t *testing.T) {
	f := newFixture(t, &fakeMail{}, defaultConfig(), nil)
	keys := f.enqueue(t, "unregistered_kind", 1)

	rep, err := f.runner.RunPass(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, rep.Failed)
	r := f.record(t, keys[0])
	assert.Equal(t, outbox.StateFailed, r.State)
	assert.Equal(t, outbox.ReasonUnknownKind, r.Reason)
	assert.Zero(t, f.mail.count())
}

func TestRunPass_RetriesTransientErrorsThenFails(t *testing.T) {
	m := &fakeMail{err: &mail.SendError{Err: errors.New("provider down")}}
	cfg := defaultConfig()
	cfg.Retry.MaxAttempts = 3
	f := newFixture(t, m, cfg, sendable())
	keys := f.enqueue(t, testKind, 1)

	rep, err := f.runner.RunPass(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, rep.Retried)
	r := f.record(t, keys[0])
	assert.Equal(t, outbox.StatePending, r.State)
	assert.Equal(t, t0.Add(time.Minute), r.NextAttemptAt)

	rep, err = f.runner.RunPass(context.Background())
	require.NoError(t, err)
	assert.Zero(t, rep.Retried, "not due before its backoff ends")

	f.clock = t0.Add(time.Minute)
	_, err = f.runner.RunPass(context.Background())
	require.NoError(t, err)
	assert.Equal(t, t0.Add(3*time.Minute), f.record(t, keys[0]).NextAttemptAt)

	f.clock = t0.Add(3 * time.Minute)
	rep, err = f.runner.RunPass(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, rep.Failed)
	r = f.record(t, keys[0])
	assert.Equal(t, outbox.StateFailed, r.State)
	assert.Equal(t, outbox.ReasonMaxAttempts, r.Reason)
	assert.Equal(t, 3, r.Attempts)
	assert.Equal(t, 3, m.count())
}

func TestRunPass_PermanentErrorFailsAtOnce(t *testing.T) {
	m := &fakeMail{err: &mail.SendError{Err: errors.New("rejected"), Permanent: true}}
	f := newFixture(t, m, defaultConfig(), sendable())
	keys := f.enqueue(t, testKind, 1)

	_, err := f.runner.RunPass(context.Background())
	require.NoError(t, err)
	r := f.record(t, keys[0])
	assert.Equal(t, outbox.StateFailed, r.State)
	assert.Equal(t, outbox.ReasonPermanent, r.Reason)
	assert.Equal(t, 1, m.count())
}

func TestRunPass_ResultAfterClaimExpiryIsNotWritten(t *testing.T) {
	m := &fakeMail{}
	f := newFixture(t, m, defaultConfig(), sendable())
	keys := f.enqueue(t, testKind, 1)
	m.onSend = func() { f.clock = t0.Add(outbox.ClaimLease) }

	rep, err := f.runner.RunPass(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, rep.Lost)
	assert.Equal(t, outbox.StateSending, f.record(t, keys[0]).State)

	m.onSend = nil
	rep, err = f.runner.RunPass(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, rep.Sweep.Expired)
	assert.Equal(t, outbox.StateUnknown, f.record(t, keys[0]).State)
	assert.Equal(t, 1, m.count(), "an unknown record is never resent")
}

func TestRunPass_StopsAtTheBudget(t *testing.T) {
	cfg := defaultConfig()
	cfg.SendsPerSecond = 10
	f := newFixture(t, &fakeMail{}, cfg, sendable())
	f.enqueue(t, testKind, 20)

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	start := time.Now()
	rep, err := f.runner.RunPass(ctx)
	require.NoError(t, err)
	assert.Less(t, time.Since(start), time.Second)
	assert.GreaterOrEqual(t, rep.Sent, 1)
	assert.LessOrEqual(t, rep.Sent, 3, "10 per second within 250ms")
}

func TestRunPass_SendsAtMostRatePerSecond(t *testing.T) {
	cfg := defaultConfig()
	cfg.SendsPerSecond = 20
	f := newFixture(t, &fakeMail{}, cfg, sendable())
	f.enqueue(t, testKind, 5)

	start := time.Now()
	rep, err := f.runner.RunPass(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 5, rep.Sent)
	assert.GreaterOrEqual(t, time.Since(start), 190*time.Millisecond, "five sends at 20 per second take four intervals of 50ms")
}

func TestRunPass_TimedOutSendIsRetriedAndThePassGoesOn(t *testing.T) {
	m := &fakeMail{}
	m.blockOnce.Store(true)
	f := newFixture(t, m, defaultConfig(), sendable())
	f.runner.sendTimeout = 50 * time.Millisecond
	keys := f.enqueue(t, testKind, 2)

	rep, err := f.runner.RunPass(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, rep.Retried)
	assert.Equal(t, 1, rep.Sent, "the pass goes on after a timed-out send")
	r := f.record(t, keys[0])
	assert.Equal(t, outbox.StatePending, r.State, "a timed-out send is transient")
	assert.Equal(t, t0.Add(time.Minute), r.NextAttemptAt)
	assert.Equal(t, outbox.StateSent, f.record(t, keys[1]).State)
}

func TestRunPass_KeepsTheRateAcrossPasses(t *testing.T) {
	var lock sync.Mutex
	var at []time.Time
	m := &fakeMail{onSend: func() {
		lock.Lock()
		defer lock.Unlock()
		at = append(at, time.Now())
	}}
	cfg := defaultConfig()
	cfg.SendsPerSecond = 5
	f := newFixture(t, m, cfg, sendable())
	f.enqueue(t, testKind, 2)

	budget, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	rep, err := f.runner.RunPass(budget)
	require.NoError(t, err)
	require.Equal(t, 1, rep.Sent, "the budget ends the first pass before its second send")
	rep, err = f.runner.RunPass(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, rep.Sent)

	require.Len(t, at, 2)
	assert.GreaterOrEqual(t, at[1].Sub(at[0]), 190*time.Millisecond, "5 per second leaves 200ms between sends, even across passes")
}

func TestShutdown_FinishesTheSendAndClaimsNothingMore(t *testing.T) {
	m := &fakeMail{sent: make(chan mail.Envelope), release: make(chan struct{})}
	f := newFixture(t, m, defaultConfig(), sendable())
	keys := f.enqueue(t, testKind, 3)

	ctx, cancel := context.WithCancel(context.Background())
	passDone := make(chan Report)
	go func() {
		rep, err := f.runner.RunPass(ctx)
		assert.NoError(t, err)
		passDone <- rep
	}()
	<-m.sent
	cancel()

	shutdownDone := make(chan error)
	go func() { shutdownDone <- f.runner.Shutdown(context.Background()) }()
	select {
	case <-shutdownDone:
		t.Fatal("Shutdown returned while a send was in flight")
	case <-time.After(50 * time.Millisecond):
	}
	close(m.release)
	require.NoError(t, <-shutdownDone)
	rep := <-passDone
	assert.Equal(t, 1, rep.Sent)

	assert.Equal(t, outbox.StateSent, f.record(t, keys[0]).State, "the in-flight result is written after cancellation")
	assert.Equal(t, outbox.StatePending, f.record(t, keys[1]).State)
	assert.Equal(t, outbox.StatePending, f.record(t, keys[2]).State)

	rep, err := f.runner.RunPass(context.Background())
	require.NoError(t, err)
	assert.False(t, rep.Ran, "no pass runs after shutdown")
}

func TestShutdown_GivesUpAtItsDeadline(t *testing.T) {
	m := &fakeMail{sent: make(chan mail.Envelope), release: make(chan struct{})}
	defer close(m.release)
	f := newFixture(t, m, defaultConfig(), sendable())
	f.enqueue(t, testKind, 1)
	go func() { _, _ = f.runner.RunPass(context.Background()) }()
	<-m.sent

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	assert.ErrorIs(t, f.runner.Shutdown(ctx), context.DeadlineExceeded)
}

func TestRunPass_OnePassAtATime(t *testing.T) {
	m := &fakeMail{sent: make(chan mail.Envelope), release: make(chan struct{})}
	f := newFixture(t, m, defaultConfig(), sendable())
	f.enqueue(t, testKind, 1)
	done := make(chan struct{})
	go func() {
		_, _ = f.runner.RunPass(context.Background())
		close(done)
	}()
	<-m.sent

	rep, err := f.runner.RunPass(context.Background())
	require.NoError(t, err)
	assert.False(t, rep.Ran)
	close(m.release)
	<-done
}

type countingHook struct {
	err  error
	name string
	runs int
}

func (h *countingHook) Name() string { return h.name }

func (h *countingHook) Run(context.Context, time.Time) (int, error) {
	h.runs++
	return 2, h.err
}

func TestRunPass_RunsRetentionHooks(t *testing.T) {
	broken := &countingHook{name: "broken", err: errors.New("boom")}
	ok := &countingHook{name: "ok"}
	f := newFixture(t, &fakeMail{noSender: true}, defaultConfig(), nil, broken, ok)

	rep, err := f.runner.RunPass(context.Background())
	require.NoError(t, err)
	assert.Equal(t, map[string]int{"ok": 2}, rep.Retained)
	assert.Equal(t, 1, broken.runs)
	assert.Equal(t, 1, ok.runs, "a failing hook does not stop the next")
}

type tickHook struct{ runs atomic.Int32 }

func (*tickHook) Name() string { return "tick" }

func (h *tickHook) Run(context.Context, time.Time) (int, error) {
	h.runs.Add(1)
	return 0, nil
}

func TestLoop_RunsPassesUntilCancelled(t *testing.T) {
	hook := &tickHook{}
	f := newFixture(t, &fakeMail{noSender: true}, defaultConfig(), nil, hook)
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		f.runner.Loop(ctx, 10*time.Millisecond)
		close(stopped)
	}()
	assert.Eventually(t, func() bool { return hook.runs.Load() >= 3 }, time.Second, 5*time.Millisecond)
	cancel()
	<-stopped
	n := hook.runs.Load()
	time.Sleep(30 * time.Millisecond)
	assert.Equal(t, n, hook.runs.Load(), "no pass after the loop stops")
}

func TestLoop_ZeroIntervalRunsNothing(t *testing.T) {
	hook := &tickHook{}
	f := newFixture(t, &fakeMail{noSender: true}, defaultConfig(), nil, hook)
	f.runner.Loop(context.Background(), 0)
	assert.Zero(t, hook.runs.Load())
}
