// Package maintenance runs passes: short, budgeted runs that sweep the mail
// outbox, run retention hooks and send due mail (ADR-0004 decision 13).
package maintenance

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"errors"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/server/pkg/applog"
	"github.com/reearth/reearth-accounts/server/pkg/mail"
	"github.com/reearth/reearth-accounts/server/pkg/outbox"
	"golang.org/x/time/rate"
)

// sendTimeout bounds one claim, send and result write. They run detached from
// the pass context, so stopping the server never cuts a send between its
// claim and its result.
const sendTimeout = 5 * time.Second

type Config struct {
	Lease time.Duration
	// Rate is the most messages a pass sends per second.
	Rate  float64
	Retry outbox.RetryPolicy
	TTL   time.Duration
}

// Resolver builds the mail for a claimed record when it is sent, reading the
// recipient's address and language then. It returns a reason and no envelope
// when the mail no longer applies. An error is retried like a send error.
type Resolver interface {
	Resolve(ctx context.Context, r outbox.Record) (*mail.Envelope, outbox.Reason, error)
}

// Retention is a cleanup step that each pass runs after the outbox sweep.
type Retention interface {
	Name() string
	Run(ctx context.Context, now time.Time) (int, error)
}

type Report struct {
	Failed   int
	Lost     int
	Ran      bool
	Retained map[string]int
	Retried  int
	Sent     int
	Sweep    outbox.SweepResult
}

type Runner struct {
	cfg        Config
	jitter     func(time.Duration) time.Duration
	mail       gateway.Mail
	now        func() time.Time
	outbox     outbox.Repo
	pass       sync.Mutex
	resolvers  map[mail.Kind]Resolver
	retentions []Retention
	stopOnce   sync.Once
	stopping   chan struct{}
	token      func() string
}

func NewRunner(cfg Config, ob outbox.Repo, m gateway.Mail, resolvers map[mail.Kind]Resolver, retentions ...Retention) *Runner {
	return &Runner{
		cfg:        cfg,
		jitter:     jitter,
		mail:       m,
		now:        func() time.Time { return time.Now().UTC() },
		outbox:     ob,
		resolvers:  resolvers,
		retentions: retentions,
		stopping:   make(chan struct{}),
		token:      newToken,
	}
}

// Loop runs a pass every interval, each within that interval, until ctx ends.
// An interval of 0 runs nothing.
func (r *Runner) Loop(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		return
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		pctx, cancel := context.WithTimeout(ctx, interval)
		rep, err := r.RunPass(pctx)
		cancel()
		logPass(ctx, rep, err)
	}
}

// RunPass does one pass within ctx's deadline. It reports Ran false when
// another pass on this instance is running or the runner is shut down.
func (r *Runner) RunPass(ctx context.Context) (Report, error) {
	if !r.pass.TryLock() {
		return Report{}, nil
	}
	defer r.pass.Unlock()
	if r.stopped() {
		return Report{}, nil
	}
	rep := Report{Ran: true, Retained: map[string]int{}}
	now := r.now()
	sweep, err := r.outbox.Sweep(ctx, now, r.cfg.TTL)
	if err != nil {
		return rep, err
	}
	rep.Sweep = sweep
	for _, h := range r.retentions {
		n, err := h.Run(ctx, now)
		if err != nil {
			applog.Warn(ctx, "maintenance retention failed", applog.Operation(h.Name()), applog.Err(err))
			continue
		}
		rep.Retained[h.Name()] = n
	}
	if !r.mail.Deliverable() {
		return rep, nil
	}
	limiter := rate.NewLimiter(rate.Limit(r.cfg.Rate), 1)
	for !r.stopped() && limiter.Wait(ctx) == nil && !r.stopped() {
		more, err := r.sendOne(ctx, &rep)
		if err != nil || !more {
			return rep, err
		}
	}
	return rep, nil
}

// Shutdown stops new claims and waits for the running pass to finish its
// send. Later passes do nothing.
func (r *Runner) Shutdown(ctx context.Context) error {
	r.stopOnce.Do(func() { close(r.stopping) })
	idle := make(chan struct{})
	go func() {
		r.pass.Lock()
		close(idle)
	}()
	select {
	case <-idle:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *Runner) deliver(ctx context.Context, rec outbox.Record) outbox.Outcome {
	res, ok := r.resolvers[rec.Kind]
	if !ok {
		return outbox.NothingToSend{Reason: outbox.ReasonUnknownKind}
	}
	env, reason, err := res.Resolve(ctx, rec)
	if err != nil {
		applog.Warn(ctx, "outbox mail could not be built", applog.MailKind(rec.Kind), applog.Err(err))
		return outbox.SendFailed{}
	}
	if env == nil {
		return outbox.NothingToSend{Reason: reason}
	}
	if err := r.mail.Send(ctx, *env); err != nil {
		var se *mail.SendError
		return outbox.SendFailed{Permanent: errors.As(err, &se) && se.Permanent}
	}
	return outbox.Delivered{}
}

func (r *Runner) sendOne(ctx context.Context, rep *Report) (bool, error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sendTimeout)
	defer cancel()
	now := r.now()
	claim := outbox.Claim{Token: r.token(), Until: now.Add(r.cfg.Lease)}
	rec, err := r.outbox.ClaimDue(ctx, now, claim)
	if err != nil || rec == nil {
		return false, err
	}
	t := outbox.Decide(*rec, r.deliver(ctx, *rec), r.cfg.Retry, r.now(), r.jitter)
	held, err := r.outbox.Complete(ctx, rec.Key, claim.Token, t, r.now())
	if err != nil {
		return false, err
	}
	switch {
	case !held:
		rep.Lost++
		applog.Warn(ctx, "outbox result not recorded: claim no longer held", applog.MailKind(rec.Kind))
	case t.To == outbox.StateSent:
		rep.Sent++
	case t.To == outbox.StatePending:
		rep.Retried++
	default:
		rep.Failed++
		applog.Warn(ctx, "outbox mail failed", applog.MailKind(rec.Kind), applog.Reason(string(t.Reason)))
	}
	return true, nil
}

func (r *Runner) stopped() bool {
	select {
	case <-r.stopping:
		return true
	default:
		return false
	}
}

func jitter(d time.Duration) time.Duration {
	return time.Duration(float64(d) * (0.8 + 0.4*rand.Float64()))
}

func logPass(ctx context.Context, rep Report, err error) {
	if err != nil {
		applog.Error(ctx, "maintenance pass failed", applog.Err(err))
		return
	}
	if !rep.Ran {
		return
	}
	applog.Info(ctx, "maintenance pass", applog.Pass(rep))
}

func newToken() string {
	b := make([]byte, 16)
	_, _ = crand.Read(b)
	return hex.EncodeToString(b)
}
