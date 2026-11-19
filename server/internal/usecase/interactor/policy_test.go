package interactor

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/pkg/legal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const policyPayload = `{"event":"policy_version_updated","timestamp":"2026-11-20T03:00:00Z","versions":{
	"tos":{"version":"1.0.0","effectiveDate":"2026-01-01T00:00:00Z","required":true,"title":{"en":"ToS"}}}}`

func applyTos(t *testing.T, r legal.VersionRepo, at time.Time, version string) {
	t.Helper()
	m := legal.Manifest{Timestamp: at, Entries: []legal.Entry{{
		Ref:         legal.Ref{Type: "tos", Version: legal.MustParseVersion(version)},
		EffectiveAt: at.Add(-time.Hour),
		Title:       legal.LocalizedText{"en": "ToS"},
	}}}
	_, err := legal.Apply(context.Background(), r, m, func() time.Time { return at })
	require.NoError(t, err)
}

func currentTos(t *testing.T, c legal.Catalog, now time.Time) string {
	t.Helper()
	if cur := c.Current("tos", now); cur != nil {
		return cur.Version.String()
	}
	return ""
}

func TestPolicyCatalog_CachesForAtMostTheTTL(t *testing.T) {
	ctx := context.Background()
	r := memory.New().PolicyVersion
	clock := time.Date(2026, 11, 20, 3, 0, 0, 0, time.UTC)
	cat := NewPolicyCatalog(r, PolicyCatalogTTL)
	cat.now = func() time.Time { return clock }

	applyTos(t, r, clock, "1.0.0")
	c, err := cat.Get(ctx)
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", currentTos(t, c, clock))

	applyTos(t, r, clock.Add(time.Second), "1.1.0")
	clock = clock.Add(59 * time.Second)
	c, err = cat.Get(ctx)
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", currentTos(t, c, clock), "served from the cache within the TTL")

	fresh, err := cat.Refresh(ctx)
	require.NoError(t, err)
	assert.Equal(t, "1.1.0", currentTos(t, fresh, clock), "Refresh reads the store")

	applyTos(t, r, clock, "1.2.0")
	clock = clock.Add(PolicyCatalogTTL)
	c, err = cat.Get(ctx)
	require.NoError(t, err)
	assert.Equal(t, "1.2.0", currentTos(t, c, clock), "reloaded once the TTL passed")
}

func TestPolicy_ReceiveWebhook(t *testing.T) {
	ctx := context.Background()
	secret := legal.NewSecret("s")
	body := []byte(policyPayload)
	now := time.Date(2026, 11, 20, 3, 1, 0, 0, time.UTC)

	t.Run("no secret is not configured", func(t *testing.T) {
		repos := memory.New()
		p := NewPolicy(repos, legal.Secret{}, NewPolicyCatalog(repos.PolicyVersion, PolicyCatalogTTL))
		_, err := p.ReceiveWebhook(ctx, bytes.NewReader(body), secret.Sign(body))
		assert.ErrorIs(t, err, legal.ErrNotConfigured)
	})

	t.Run("applied payload refreshes this instance's catalog", func(t *testing.T) {
		repos := memory.New()
		cat := NewPolicyCatalog(repos.PolicyVersion, PolicyCatalogTTL)
		cat.now = func() time.Time { return now }
		p := NewPolicy(repos, secret, cat)
		p.now = func() time.Time { return now }

		before, err := cat.Get(ctx)
		require.NoError(t, err)
		assert.Empty(t, before.Types())

		res, err := p.ReceiveWebhook(ctx, bytes.NewReader(body), secret.Sign(body))
		require.NoError(t, err)
		assert.Equal(t, []legal.Type{"tos"}, res.Applied)

		after, err := cat.Get(ctx)
		require.NoError(t, err)
		assert.Equal(t, "1.0.0", currentTos(t, after, now))
	})

	t.Run("a missing or none signature is refused before the body is read", func(t *testing.T) {
		repos := memory.New()
		p := NewPolicy(repos, secret, NewPolicyCatalog(repos.PolicyVersion, PolicyCatalogTTL))
		for _, sig := range []string{"", "none"} {
			_, err := p.ReceiveWebhook(ctx, unreadable{t}, sig)
			assert.ErrorIs(t, err, legal.ErrBadSignature, "signature %q", sig)
		}
	})

	t.Run("a bad signature stores nothing", func(t *testing.T) {
		repos := memory.New()
		p := NewPolicy(repos, secret, NewPolicyCatalog(repos.PolicyVersion, PolicyCatalogTTL))
		p.now = func() time.Time { return now }
		_, err := p.ReceiveWebhook(ctx, bytes.NewReader(body), "none")
		assert.ErrorIs(t, err, legal.ErrBadSignature)
		heads, err := repos.PolicyVersion.FindHeads(ctx)
		require.NoError(t, err)
		assert.Empty(t, heads)
	})
}

type unreadable struct{ t *testing.T }

func (u unreadable) Read([]byte) (int, error) {
	u.t.Error("the body was read")
	return 0, io.EOF
}

func TestPolicy_ReceiveWebhookSameTimestampEitherOrder(t *testing.T) {
	ctx := context.Background()
	secret := legal.NewSecret("s")
	now := time.Date(2026, 11, 20, 3, 1, 0, 0, time.UTC)
	payload := func(version, eff string) []byte {
		return []byte(`{"event":"policy_version_updated","timestamp":"2026-11-20T03:00:00Z","versions":{"tos":{"version":"` + version + `","effectiveDate":"` + eff + `","required":true,"title":{"en":"x"}}}}`)
	}
	base := payload("1.0.0", "2026-01-01T00:00:00Z")
	inEffect := payload("1.0.5", "2026-11-19T00:00:00Z")
	upcoming := payload("1.1.0", "2026-11-21T00:00:00Z")
	run := func(order ...[]byte) string {
		repos := memory.New()
		p := NewPolicy(repos, secret, NewPolicyCatalog(repos.PolicyVersion, PolicyCatalogTTL))
		p.now = func() time.Time { return now }
		for _, b := range append([][]byte{base}, order...) {
			_, err := p.ReceiveWebhook(ctx, bytes.NewReader(b), secret.Sign(b))
			require.NoError(t, err)
		}
		c, err := p.catalog.Get(ctx)
		require.NoError(t, err)
		up := ""
		if u := c.Upcoming("tos", now); u != nil {
			up = u.Version.String()
		}
		return currentTos(t, c, now) + " / " + up
	}
	assert.Equal(t, "1.0.5 / 1.1.0", run(inEffect, upcoming))
	assert.Equal(t, "1.0.5 / 1.1.0", run(upcoming, inEffect))
}
