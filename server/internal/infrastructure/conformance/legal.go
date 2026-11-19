package conformance

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/legal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var legalT0 = time.Date(2026, 11, 20, 3, 0, 0, 0, time.UTC)

func runLegal(t *testing.T, nc Factory) {
	t.Run("Legal_HeadCompareAndSwap", func(t *testing.T) { testLegalHeadCAS(t, nc) })
	t.Run("Legal_UpsertVersionGuard", func(t *testing.T) { testLegalUpsertVersion(t, nc) })
	t.Run("Legal_ApplyBothOrders", func(t *testing.T) { testLegalApplyBothOrders(t, nc) })
	t.Run("Legal_ApplyRace", func(t *testing.T) { testLegalApplyRace(t, nc) })
	t.Run("Legal_ApplyReplayRace", func(t *testing.T) { testLegalApplyReplayRace(t, nc) })
}

func legalEntry(typ, version string, eff time.Time, title string) legal.Entry {
	return legal.Entry{
		Ref:         legal.Ref{Type: legal.Type(typ), Version: legal.MustParseVersion(version)},
		EffectiveAt: eff,
		Required:    true,
		Title:       legal.LocalizedText{"en": title, "ja": title + " (ja)"},
		URL:         legal.LocalizedText{"en": "https://example.com/" + typ},
	}
}

func legalManifest(at time.Time, entries ...legal.Entry) legal.Manifest {
	return legal.Manifest{Timestamp: at, Entries: entries}
}

func clockAt(t time.Time) func() time.Time { return func() time.Time { return t } }

func testLegalHeadCAS(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	r := c.PolicyVersion

	empty, err := r.FindHead(ctx, "tos")
	require.NoError(t, err)
	assert.Equal(t, legal.TypeHead{Type: "tos"}, empty)

	first := legal.TypeHead{
		Type: "tos", Revision: 1, AppliedAt: legalT0,
		Upcoming: &legal.Scheduled{Version: legal.MustParseVersion("2.0.0"), EffectiveAt: legalT0.Add(48 * time.Hour)},
	}
	won, err := r.CompareAndSwapHead(ctx, first)
	require.NoError(t, err)
	assert.True(t, won)

	won, err = r.CompareAndSwapHead(ctx, first)
	require.NoError(t, err)
	assert.False(t, won, "a second first write loses")

	second := legal.TypeHead{
		Type: "tos", Revision: 2, AppliedAt: legalT0.Add(time.Hour),
		Withdrawn: []legal.Withdrawal{{Version: legal.MustParseVersion("2.0.0"), At: legalT0.Add(time.Hour)}},
	}
	won, err = r.CompareAndSwapHead(ctx, second)
	require.NoError(t, err)
	assert.True(t, won)

	stale := second
	stale.Revision = 2
	won, err = r.CompareAndSwapHead(ctx, stale)
	require.NoError(t, err)
	assert.False(t, won, "a write based on a stale revision loses")

	got, err := r.FindHead(ctx, "tos")
	require.NoError(t, err)
	assertHeadEqual(t, second, got)

	heads, err := r.FindHeads(ctx)
	require.NoError(t, err)
	require.Len(t, heads, 1)
	assertHeadEqual(t, second, heads[0])
}

func assertHeadEqual(t *testing.T, want, got legal.TypeHead) {
	t.Helper()
	assert.Equal(t, want.Type, got.Type)
	assert.Equal(t, want.Revision, got.Revision)
	assert.True(t, want.AppliedAt.Equal(got.AppliedAt), "appliedAt %v != %v", want.AppliedAt, got.AppliedAt)
	if want.Upcoming == nil {
		assert.Nil(t, got.Upcoming)
	} else if assert.NotNil(t, got.Upcoming) {
		assert.Equal(t, want.Upcoming.Version, got.Upcoming.Version)
		assert.True(t, want.Upcoming.EffectiveAt.Equal(got.Upcoming.EffectiveAt))
	}
	require.Len(t, got.Withdrawn, len(want.Withdrawn))
	for i := range want.Withdrawn {
		assert.Equal(t, want.Withdrawn[i].Version, got.Withdrawn[i].Version)
		assert.True(t, want.Withdrawn[i].At.Equal(got.Withdrawn[i].At))
	}
}

func testLegalUpsertVersion(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	r := c.PolicyVersion
	now := legalT0
	eff := legalT0.Add(48 * time.Hour)

	_, err := r.CompareAndSwapHead(ctx, legal.TypeHead{Type: "tos", Revision: 1, AppliedAt: legalT0})
	require.NoError(t, err)

	find := func() legal.VersionRecord {
		t.Helper()
		vs, err := r.FindVersions(ctx)
		require.NoError(t, err)
		require.Len(t, vs, 1)
		return vs[0]
	}

	v1 := legal.NewVersionRecord(legalEntry("tos", "2.0.0", eff, "draft"), legalT0)
	require.NoError(t, r.UpsertVersion(ctx, v1, now))
	got := find()
	assert.Equal(t, v1.Ref, got.Ref)
	assert.True(t, got.EffectiveAt.Equal(eff))
	assert.True(t, got.Stamp.Equal(legalT0))
	assert.True(t, got.Required)
	assert.Equal(t, v1.Title, got.Title)
	assert.Equal(t, v1.URL, got.URL)

	newer := legal.NewVersionRecord(legalEntry("tos", "2.0.0", eff.Add(time.Hour), "final"), legalT0.Add(time.Minute))
	require.NoError(t, r.UpsertVersion(ctx, newer, now))
	assert.Equal(t, "final", find().Title["en"], "a newer write replaces a record not in effect")

	older := legal.NewVersionRecord(legalEntry("tos", "2.0.0", eff, "stale"), legalT0)
	require.NoError(t, r.UpsertVersion(ctx, older, now))
	assert.Equal(t, "final", find().Title["en"], "an older write keeps the record")

	later := legal.NewVersionRecord(legalEntry("tos", "2.0.0", eff, "after effect"), legalT0.Add(time.Hour))
	require.NoError(t, r.UpsertVersion(ctx, later, eff.Add(2*time.Hour)))
	got = find()
	assert.Equal(t, "final", got.Title["en"], "a record in effect never changes")
	assert.True(t, got.EffectiveAt.Equal(eff.Add(time.Hour)))
}

type catalogView struct {
	current, upcoming string
	upcomingAt        time.Time
	acceptable        map[string]bool
}

func viewOf(t *testing.T, ctx context.Context, r legal.VersionRepo, typ legal.Type, now time.Time, versions ...string) catalogView {
	t.Helper()
	heads, err := r.FindHeads(ctx)
	require.NoError(t, err)
	records, err := r.FindVersions(ctx)
	require.NoError(t, err)
	cat := legal.NewCatalog(heads, records)
	v := catalogView{acceptable: map[string]bool{}}
	if cur := cat.Current(typ, now); cur != nil {
		v.current = cur.Version.String()
	}
	if up := cat.Upcoming(typ, now); up != nil {
		v.upcoming = up.Version.String()
		v.upcomingAt = up.EffectiveAt.UTC()
	}
	for _, s := range versions {
		v.acceptable[s] = s == v.current || s == v.upcoming
	}
	return v
}

func testLegalApplyBothOrders(t *testing.T, nc Factory) {
	day := 24 * time.Hour
	now := legalT0.Add(time.Hour)
	base := legalManifest(legalT0.Add(-day), legalEntry("tos", "1.0.0", legalT0.Add(-30*day), "v1"))
	pairs := []struct {
		name         string
		older, newer legal.Manifest
		current      string
		upcoming     string
		upcomingAt   time.Time
	}{
		{
			name:    "revert withdraws the upcoming version",
			older:   legalManifest(legalT0, legalEntry("tos", "2.0.0", legalT0.Add(30*day), "v2")),
			newer:   legalManifest(legalT0.Add(time.Minute), legalEntry("tos", "1.0.0", legalT0.Add(-30*day), "v1")),
			current: "1.0.0",
		},
		{
			name:       "correction of the upcoming date",
			older:      legalManifest(legalT0, legalEntry("tos", "2.0.0", legalT0.Add(30*day), "v2")),
			newer:      legalManifest(legalT0.Add(time.Minute), legalEntry("tos", "2.0.0", legalT0.Add(40*day), "v2")),
			current:    "1.0.0",
			upcoming:   "2.0.0",
			upcomingAt: legalT0.Add(40 * day),
		},
		{
			name:       "a newer upcoming version replaces the older one",
			older:      legalManifest(legalT0, legalEntry("tos", "2.0.0", legalT0.Add(30*day), "v2")),
			newer:      legalManifest(legalT0.Add(time.Minute), legalEntry("tos", "3.0.0", legalT0.Add(30*day), "v3")),
			current:    "1.0.0",
			upcoming:   "3.0.0",
			upcomingAt: legalT0.Add(30 * day),
		},
	}
	for _, p := range pairs {
		t.Run(p.name, func(t *testing.T) {
			var views []catalogView
			for _, order := range [][]legal.Manifest{{base, p.older, p.newer}, {base, p.newer, p.older}} {
				c, _, done := nc(t)
				ctx := context.Background()
				for _, m := range order {
					_, err := legal.Apply(ctx, c.PolicyVersion, m, clockAt(now))
					require.NoError(t, err)
				}
				views = append(views, viewOf(t, ctx, c.PolicyVersion, "tos", now, "1.0.0", "2.0.0", "3.0.0"))
				head, err := c.PolicyVersion.FindHead(ctx, "tos")
				require.NoError(t, err)
				assert.True(t, head.AppliedAt.Equal(p.newer.Timestamp))
				done()
			}
			assert.Equal(t, p.current, views[0].current)
			assert.Equal(t, p.upcoming, views[0].upcoming)
			assert.True(t, p.upcomingAt.Equal(views[0].upcomingAt))
			assert.Equal(t, views[0], views[1], "both delivery orders end in the same state")
		})
	}
}

// testLegalApplyRace delivers 16 payloads at once, each naming its own
// upcoming version. The newest wins and every other stored version is
// withdrawn.
func testLegalApplyRace(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	r := c.PolicyVersion
	const n = 16
	now := legalT0.Add(time.Hour)
	eff := legalT0.Add(30 * 24 * time.Hour)

	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m := legalManifest(legalT0.Add(time.Duration(i)*time.Second), legalEntry("tos", fmt.Sprintf("2.%d.0", i), eff, "v"))
			_, err := legal.Apply(ctx, r, m, clockAt(now))
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	head, err := r.FindHead(ctx, "tos")
	require.NoError(t, err)
	assert.True(t, head.AppliedAt.Equal(legalT0.Add((n-1)*time.Second)))

	heads, err := r.FindHeads(ctx)
	require.NoError(t, err)
	records, err := r.FindVersions(ctx)
	require.NoError(t, err)
	cat := legal.NewCatalog(heads, records)
	up := cat.Upcoming("tos", now)
	require.NotNil(t, up)
	assert.Equal(t, fmt.Sprintf("2.%d.0", n-1), up.Version.String())
	if cur := cat.Current("tos", eff.Add(time.Hour)); assert.NotNil(t, cur) {
		assert.Equal(t, up.Version, cur.Version, "only the winner takes effect")
	}
	for _, rec := range records {
		if rec.Version == up.Version {
			continue
		}
		assert.True(t, cat.Withdrawn(rec.Ref), "stored %s must be withdrawn", rec.Version)
	}
}

// testLegalApplyReplayRace delivers the same payload 16 times at once. The
// head is written once and the version stored once.
func testLegalApplyReplayRace(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	r := c.PolicyVersion
	const n = 16
	m := legalManifest(legalT0, legalEntry("tos", "1.0.0", legalT0.Add(-time.Hour), "v1"))

	var wg sync.WaitGroup
	applied := make(chan int, n)
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := legal.Apply(ctx, r, m, clockAt(legalT0))
			assert.NoError(t, err)
			applied <- len(res.Applied)
		}()
	}
	wg.Wait()
	close(applied)
	total := 0
	for a := range applied {
		total += a
	}
	assert.Equal(t, 1, total, "exactly one delivery applies")

	head, err := r.FindHead(ctx, "tos")
	require.NoError(t, err)
	assert.Equal(t, int64(1), head.Revision)
	records, err := r.FindVersions(ctx)
	require.NoError(t, err)
	assert.Len(t, records, 1)
}
