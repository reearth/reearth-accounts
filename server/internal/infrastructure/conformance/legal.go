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

var (
	legalApplied = legal.Applied{Digest: []byte{1}, Version: legal.MustParseVersion("2.0.0")}
	legalT0      = time.Date(2026, 11, 20, 3, 0, 0, 0, time.UTC)
)

func runLegal(t *testing.T, nc Factory) {
	t.Run("Legal_HeadCompareAndSwap", func(t *testing.T) { testLegalHeadCAS(t, nc) })
	t.Run("Legal_ArchiveVersionsGuard", func(t *testing.T) { testLegalArchiveVersions(t, nc) })
	t.Run("Legal_ApplyBothOrders", func(t *testing.T) { testLegalApplyBothOrders(t, nc) })
	t.Run("Legal_ApplyRace", func(t *testing.T) { testLegalApplyRace(t, nc) })
	t.Run("Legal_ApplyReplayRace", func(t *testing.T) { testLegalApplyReplayRace(t, nc) })
	runLegalFaults(t, nc)
	runLegalReceived(t, nc)
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

func legalVersions(vs ...legal.HeadVersion) map[legal.Version]legal.HeadVersion {
	m := make(map[legal.Version]legal.HeadVersion, len(vs))
	for _, v := range vs {
		m[v.Version] = v
	}
	return m
}

func legalHeadVersion(typ, version string, eff time.Time, title string, withdrawnAt *time.Time) legal.HeadVersion {
	e := legalEntry(typ, version, eff, title)
	return legal.HeadVersion{
		VersionRecord: legal.VersionRecord(e),
		WithdrawnAt:   withdrawnAt,
	}
}

func testLegalHeadCAS(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	r := c.PolicyVersion

	empty, err := r.FindHead(ctx, "tos")
	require.NoError(t, err)
	assert.Equal(t, legal.TypeHead{Type: "tos"}, empty)

	first := legal.TypeHead{
		Applied:   legal.Applied{Digest: []byte{1, 2, 3}, Version: legal.MustParseVersion("2.0.0")},
		AppliedAt: legalT0,
		Revision:  1,
		Type:      "tos",
		Versions: legalVersions(
			legalHeadVersion("tos", "1.0.0", legalT0.Add(-48*time.Hour), "v1", nil),
			legalHeadVersion("tos", "2.0.0", legalT0.Add(48*time.Hour), "v2", nil),
		),
	}
	won, err := r.CompareAndSwapHead(ctx, first)
	require.NoError(t, err)
	assert.True(t, won)

	won, err = r.CompareAndSwapHead(ctx, first)
	require.NoError(t, err)
	assert.False(t, won, "a second first write loses")

	withdrawnAt := legalT0.Add(time.Hour + 123*time.Millisecond)
	second := legal.TypeHead{
		Applied:   legal.Applied{Digest: []byte{4}, Version: legal.MustParseVersion("1.0.0")},
		AppliedAt: legalT0.Add(time.Hour),
		Revision:  2,
		Type:      "tos",
		Versions: legalVersions(
			legalHeadVersion("tos", "1.0.0", legalT0.Add(-48*time.Hour), "v1", nil),
			legalHeadVersion("tos", "2.0.0", legalT0.Add(48*time.Hour), "v2", &withdrawnAt),
		),
	}
	won, err = r.CompareAndSwapHead(ctx, second)
	require.NoError(t, err)
	assert.True(t, won)

	got, err := r.FindHead(ctx, "tos")
	require.NoError(t, err)
	assertHeadEqual(t, second, got)

	won, err = r.CompareAndSwapHead(ctx, second)
	require.NoError(t, err)
	assert.False(t, won, "a write based on a stale revision loses")

	heads, err := r.FindHeads(ctx)
	require.NoError(t, err)
	require.Len(t, heads, 1)
	assertHeadEqual(t, second, heads[0])

	third := first
	third.Revision = 3
	won, err = r.CompareAndSwapHead(ctx, third)
	require.NoError(t, err)
	require.True(t, won)
	got, err = r.FindHead(ctx, "tos")
	require.NoError(t, err)
	assertHeadEqual(t, third, got)
}

func assertHeadEqual(t *testing.T, want, got legal.TypeHead) {
	t.Helper()
	assert.Equal(t, want.Type, got.Type)
	assert.Equal(t, want.Revision, got.Revision)
	assert.Equal(t, want.Applied, got.Applied)
	assert.True(t, want.AppliedAt.Equal(got.AppliedAt), "appliedAt %v != %v", want.AppliedAt, got.AppliedAt)
	require.Len(t, got.Versions, len(want.Versions))
	for v, w := range want.Versions {
		g, ok := got.Versions[v]
		if !assert.True(t, ok, "version %s", v) {
			continue
		}
		assertRecordEqual(t, &w.VersionRecord, &g.VersionRecord)
		if w.WithdrawnAt == nil {
			assert.Nil(t, g.WithdrawnAt)
		} else if assert.NotNil(t, g.WithdrawnAt) {
			assert.True(t, w.WithdrawnAt.Equal(*g.WithdrawnAt), "withdrawnAt %v != %v", w.WithdrawnAt, g.WithdrawnAt)
		}
	}
}

func assertRecordEqual(t *testing.T, want, got *legal.VersionRecord) {
	t.Helper()
	if want == nil {
		assert.Nil(t, got)
		return
	}
	if !assert.NotNil(t, got) {
		return
	}
	assert.Equal(t, want.Ref, got.Ref)
	assert.True(t, want.EffectiveAt.Equal(got.EffectiveAt), "effectiveAt %v != %v", want.EffectiveAt, got.EffectiveAt)
	assert.Equal(t, want.Required, got.Required)
	assert.Equal(t, want.Title, got.Title)
	assert.Equal(t, want.URL, got.URL)
}

func testLegalArchiveVersions(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	r := c.PolicyVersion
	eff := legalT0.Add(48 * time.Hour)

	head := func(rev int64, title string) legal.TypeHead {
		return legal.TypeHead{Type: "tos", Revision: rev, Applied: legalApplied, AppliedAt: legalT0, Versions: legalVersions(legalHeadVersion("tos", "2.0.0", eff, title, nil))}
	}
	won, err := r.CompareAndSwapHead(ctx, head(1, "draft"))
	require.NoError(t, err)
	require.True(t, won)

	find := func() legal.VersionRecord {
		t.Helper()
		vs, err := r.FindVersions(ctx)
		require.NoError(t, err)
		require.Len(t, vs, 1)
		return vs[0]
	}

	require.NoError(t, r.ArchiveVersions(ctx, head(2, "draft")))
	got := find()
	want := head(2, "draft").Versions[legal.MustParseVersion("2.0.0")].VersionRecord
	assertRecordEqual(t, &want, &got)

	require.NoError(t, r.ArchiveVersions(ctx, head(3, "final")))
	assert.Equal(t, "final", find().Title["en"], "a newer revision replaces the record")

	require.NoError(t, r.ArchiveVersions(ctx, head(2, "stale")))
	assert.Equal(t, "final", find().Title["en"], "an older revision keeps the record")

	require.NoError(t, r.ArchiveVersions(ctx, head(3, "final")))
	assert.Equal(t, "final", find().Title["en"], "the same revision is idempotent")
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
	cat := legal.NewCatalog(heads)
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
	cat := legal.NewCatalog(heads)
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
