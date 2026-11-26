package ratelimit

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/log"
	"github.com/stretchr/testify/assert"
)

func newTestLimiter(enforce bool) (*Limiter, *time.Time) {
	l := New(Config{Enforce: enforce, PerMinute: 60, Burst: 2})
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return now }
	return l, &now
}

func logCtx() (context.Context, *bytes.Buffer) {
	var buf bytes.Buffer
	return log.ContextWith(context.Background(), log.NewWithOutput(&buf)), &buf
}

func TestLimiter_LogOnlyAllowsAndLogs(t *testing.T) {
	l, _ := newTestLimiter(false)
	ctx, logs := logCtx()

	assert.True(t, l.Allow(ctx, "user:a", "op", 2))
	assert.Empty(t, logs.String())

	assert.True(t, l.Allow(ctx, "user:a", "op", 1))
	out := logs.String()
	assert.Contains(t, out, "user lookup rate limit exceeded")
	assert.Contains(t, out, `"limit_key": "user:a"`)
	assert.Contains(t, out, `"operation": "op"`)
	assert.Contains(t, out, `"count": 1`)
	assert.Contains(t, out, `"enforced": false`)
}

func TestLimiter_EnforceRejects(t *testing.T) {
	l, now := newTestLimiter(true)
	ctx, logs := logCtx()

	assert.True(t, l.Allow(ctx, "user:a", "op", 2))
	assert.False(t, l.Allow(ctx, "user:a", "op", 1))
	assert.Contains(t, logs.String(), `"enforced": true`)

	*now = now.Add(time.Second)
	assert.True(t, l.Allow(ctx, "user:a", "op", 1), "one token refills per second at 60/min")
}

func TestLimiter_ViewersHaveSeparateBudgets(t *testing.T) {
	l, _ := newTestLimiter(true)
	ctx := context.Background()

	assert.True(t, l.Allow(ctx, "user:a", "op", 2))
	assert.False(t, l.Allow(ctx, "user:a", "op", 1))
	assert.True(t, l.Allow(ctx, "user:b", "op", 2))
	assert.True(t, l.Allow(ctx, "ip:192.0.2.1", "op", 2))
}

func TestLimiter_DisabledAllowsEverything(t *testing.T) {
	var l *Limiter = New(Config{PerMinute: 0, Enforce: true})
	assert.True(t, l.Allow(context.Background(), "user:a", "op", 1000))
}

func TestViewerKey(t *testing.T) {
	a := user.New().NewID().Name("a").Email("a@example.com").MustBuild()
	b := user.New().NewID().Name("b").Email("b@example.com").MustBuild()
	c := &workspace.Client{ID: "svc", Name: "svc"}

	assert.Equal(t, "client:svc", ViewerKey(c, a, "10.0.0.1"))
	assert.Equal(t, ViewerKey(c, a, "10.0.0.1"), ViewerKey(c, b, "10.0.0.2"), "acting for another user shares the client's bucket")
	assert.Equal(t, ViewerKey(c, nil, "10.0.0.1"), ViewerKey(c, a, "10.0.0.1"))
	assert.Equal(t, "user:"+a.ID().String(), ViewerKey(nil, a, "10.0.0.1"))
	assert.Equal(t, "ip:10.0.0.1", ViewerKey(nil, nil, "10.0.0.1"))
}
