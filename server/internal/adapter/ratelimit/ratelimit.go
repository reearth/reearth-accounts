// Package ratelimit limits how many users one viewer can look up (ADR-0003
// decision 6). Buckets are in memory and per instance: with N replicas a
// viewer gets up to N times the configured budget. The load balancer limits
// per IP across instances.
package ratelimit

import (
	"context"
	"sync"
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/applog"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"golang.org/x/time/rate"
)

const (
	idleTTL       = 10 * time.Minute
	sweepInterval = time.Minute
)

type Config struct {
	// Enforce rejects over-budget lookups. When false, they are only logged.
	Enforce   bool `default:"false"`
	PerMinute int  `default:"120"`
	Burst     int  `default:"60"`
}

type Limiter struct {
	enforce bool
	limit   rate.Limit
	burst   int
	now     func() time.Time

	mu        sync.Mutex
	viewers   map[string]*bucket
	lastSweep time.Time
}

type bucket struct {
	tokens *rate.Limiter
	seen   time.Time
}

// New returns nil, which allows everything, when PerMinute is not positive.
func New(cfg Config) *Limiter {
	if cfg.PerMinute <= 0 {
		return nil
	}
	return &Limiter{
		enforce: cfg.Enforce,
		limit:   rate.Limit(float64(cfg.PerMinute) / 60),
		burst:   max(cfg.Burst, 1),
		now:     time.Now,
		viewers: map[string]*bucket{},
	}
}

// Allow takes n lookups from the viewer's budget. An over-budget call is
// logged, and rejected only in enforce mode.
func (l *Limiter) Allow(ctx context.Context, viewer, op string, n int) bool {
	if l == nil || n <= 0 {
		return true
	}
	if l.take(viewer, n) {
		return true
	}
	applog.Warn(ctx, "user lookup rate limit exceeded", applog.LimitKey(viewer), applog.Operation(op), applog.Count(n), applog.Enforced(l.enforce))
	return !l.enforce
}

func (l *Limiter) take(viewer string, n int) bool {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	if now.Sub(l.lastSweep) > sweepInterval {
		for k, b := range l.viewers {
			if now.Sub(b.seen) > idleTTL {
				delete(l.viewers, k)
			}
		}
		l.lastSweep = now
	}

	b, ok := l.viewers[viewer]
	if !ok {
		b = &bucket{tokens: rate.NewLimiter(l.limit, l.burst)}
		l.viewers[viewer] = b
	}
	b.seen = now
	return b.tokens.AllowN(now, n)
}

// ViewerKey identifies who is looking users up: the signed-in user, or the
// client IP for an anonymous caller.
func ViewerKey(u *user.User, clientIP string) string {
	if u != nil {
		return "user:" + u.ID().String()
	}
	return "ip:" + clientIP
}
