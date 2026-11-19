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

const (
	sendTimeout   = 5 * time.Second
	resultTimeout = 2 * time.Second
)

type Config struct {
	Lease          time.Duration
	Retry          outbox.RetryPolicy
	SendsPerSecond float64
	TTL            time.Duration
}

type Resolver interface {
	Resolve(ctx context.Context, r outbox.Record) (*mail.Envelope, outbox.Reason, error)
}

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
	cfg         Config
	jitter      func(time.Duration) time.Duration
	limiter     *rate.Limiter
	mail        gateway.Mail
	now         func() time.Time
	outbox      outbox.Repo
	pass        sync.Mutex
	resolvers   map[mail.Kind]Resolver
	retentions  []Retention
	sendTimeout time.Duration
	stopOnce    sync.Once
	stopping    chan struct{}
	token       func() string
}

func NewRunner(cfg Config, ob outbox.Repo, m gateway.Mail, resolvers map[mail.Kind]Resolver, retentions ...Retention) *Runner {
	return &Runner{
		cfg:         cfg,
		jitter:      jitter,
		limiter:     rate.NewLimiter(rate.Limit(cfg.SendsPerSecond), 1),
		mail:        m,
		now:         func() time.Time { return time.Now().UTC() },
		outbox:      ob,
		resolvers:   resolvers,
		retentions:  retentions,
		sendTimeout: sendTimeout,
		stopping:    make(chan struct{}),
		token:       newToken,
	}
}

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
	for !r.stopped() && r.limiter.Wait(ctx) == nil && !r.stopped() {
		more, err := r.sendOne(ctx, &rep)
		if err != nil || !more {
			return rep, err
		}
	}
	return rep, nil
}

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

// detached outlives the pass context, so stopping a pass never cuts a send
// between its claim and its result.
func detached(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), timeout)
}

func (r *Runner) sendOne(ctx context.Context, rep *Report) (bool, error) {
	sendCtx, cancelSend := detached(ctx, r.sendTimeout)
	defer cancelSend()
	now := r.now()
	claim := outbox.Claim{Token: r.token(), Until: now.Add(r.cfg.Lease)}
	rec, err := r.outbox.ClaimDue(sendCtx, now, claim)
	if err != nil || rec == nil {
		return false, err
	}
	t := outbox.Decide(*rec, r.deliver(sendCtx, *rec), r.cfg.Retry, r.now(), r.jitter)
	resultCtx, cancelResult := detached(ctx, resultTimeout)
	defer cancelResult()
	held, err := r.outbox.Complete(resultCtx, rec.Key, claim.Token, t, r.now())
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
