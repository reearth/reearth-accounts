package conformance

import (
	"context"
	"fmt"
	"math/rand"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/legal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runLegalReceived(t *testing.T, nc Factory) {
	t.Run("Legal_SupersededVersionRepublished", func(t *testing.T) { testLegalSupersededRepublished(t, nc) })
	t.Run("Legal_CorrectionBeforeCurrentDate", func(t *testing.T) { testLegalCorrectionBeforeCurrent(t, nc) })
	t.Run("Legal_VersionOlderThanCurrentIsKept", func(t *testing.T) { testLegalOlderThanCurrentKept(t, nc) })
	t.Run("Legal_StaleUpcomingThenRepublish", func(t *testing.T) { testLegalStaleUpcomingThenRepublish(t, nc) })
	t.Run("Legal_TieWithInEffectLoser", func(t *testing.T) { testLegalTieInEffectLoser(t, nc) })
	t.Run("Legal_TieBothUpcoming", func(t *testing.T) { testLegalTieBothUpcoming(t, nc) })
	t.Run("Legal_ArchiveKeepsWithdrawnContent", func(t *testing.T) { testLegalArchiveKeepsWithdrawnContent(t, nc) })
	t.Run("Legal_ConcurrentMatchesSequential", func(t *testing.T) { testLegalConcurrentMatchesSequential(t, nc) })
	t.Run("Legal_RenamedWithdrawnVersionIsOrderDependent", func(t *testing.T) { testLegalRenamedWithdrawn(t, nc) })
}

// legalState renders what users can observe for tos at now: current and
// upcoming with their content, and which of versions are withdrawn.
func legalState(t *testing.T, r legal.VersionRepo, now time.Time, versions ...string) string {
	t.Helper()
	cat := legalCatalog(t, r)
	show := func(v *legal.VersionRecord) string {
		if v == nil {
			return "-"
		}
		return fmt.Sprintf("%s@%s %q", v.Version, v.EffectiveAt.UTC().Format(time.RFC3339), v.Title["en"])
	}
	var withdrawn []string
	for _, v := range versions {
		if cat.Withdrawn(legal.Ref{Type: "tos", Version: legal.MustParseVersion(v)}) {
			withdrawn = append(withdrawn, v)
		}
	}
	return fmt.Sprintf("current=%s upcoming=%s withdrawn=%v", show(cat.Current("tos", now)), show(cat.Upcoming("tos", now)), withdrawn)
}

func applyLegal(t *testing.T, r legal.VersionRepo, now time.Time, ms ...legal.Manifest) {
	t.Helper()
	for _, m := range ms {
		_, err := legal.Apply(context.Background(), r, m, clockAt(now))
		require.NoError(t, err)
	}
}

func bothOrders(t *testing.T, nc Factory, now, readAt time.Time, base []legal.Manifest, a, b legal.Manifest, versions ...string) (string, string) {
	t.Helper()
	var out []string
	for _, order := range [][]legal.Manifest{{a, b}, {b, a}} {
		c, _, done := nc(t)
		applyLegal(t, c.PolicyVersion, now, base...)
		applyLegal(t, c.PolicyVersion, now, order...)
		out = append(out, legalState(t, c.PolicyVersion, readAt, versions...))
		done()
	}
	return out[0], out[1]
}

func archived(t *testing.T, r legal.VersionRepo) map[string]legal.VersionRecord {
	t.Helper()
	recs, err := r.FindVersions(context.Background())
	require.NoError(t, err)
	m := map[string]legal.VersionRecord{}
	for _, rec := range recs {
		m[rec.Version.String()] = rec
	}
	return m
}

func testLegalSupersededRepublished(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	r := c.PolicyVersion
	day := func(d int) time.Time { return legalT0.Add(time.Duration(d) * legalDay) }
	applyLegal(t, r, day(0), legalManifest(day(0), legalEntry("tos", "1.0.0", day(0), "v1")))
	applyLegal(t, r, day(1), legalManifest(day(1), legalEntry("tos", "2.0.0", day(10), "v2")))
	applyLegal(t, r, day(20), legalManifest(day(20), legalEntry("tos", "1.0.0", day(30), "rewritten")))

	for _, at := range []time.Time{day(20), day(31)} {
		assert.Equal(t, `current=2.0.0@`+day(10).Format(time.RFC3339)+` "v2" upcoming=- withdrawn=[]`, legalState(t, r, at, "1.0.0", "2.0.0"), "current never moves back")
	}
	v1 := archived(t, r)["1.0.0"]
	assert.Equal(t, "v1", v1.Title["en"], "a version in effect keeps its content")
	assert.True(t, v1.EffectiveAt.Equal(day(0)))
}

func testLegalCorrectionBeforeCurrent(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	r := c.PolicyVersion
	applyLegal(t, r, legalT0, legalBase,
		legalManifest(legalT0, legalEntry("tos", "2.0.0", legalT0.Add(30*legalDay), "v2")),
		legalManifest(legalT0.Add(time.Minute), legalEntry("tos", "2.0.0", legalT0.Add(-40*legalDay), "v2")),
	)
	cat := legalCatalog(t, r)
	assert.False(t, cat.Withdrawn(legal.Ref{Type: "tos", Version: legal.MustParseVersion("2.0.0")}), "the correction does not withdraw 2.0.0")
	assert.Equal(t, "1.0.0", legalVersion(cat.Current("tos", legalT0.Add(60*legalDay))), "2.0.0 is older than the current date, so it never becomes current")
	assert.Nil(t, cat.Upcoming("tos", legalT0))
	assert.True(t, archived(t, r)["2.0.0"].EffectiveAt.Equal(legalT0.Add(-40*legalDay)))
}

func testLegalOlderThanCurrentKept(t *testing.T, nc Factory) {
	for _, tc := range []struct {
		name string
		at   time.Time
	}{
		{"from a newer payload", legalT0.Add(time.Hour)},
		{"from an older payload", legalT0.Add(-time.Hour)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _, done := nc(t)
			defer done()
			r := c.PolicyVersion
			applyLegal(t, r, legalT0, legalManifest(legalT0, legalEntry("tos", "2.0.0", legalT0.Add(-legalDay), "v2")))
			applyLegal(t, r, legalT0.Add(2*time.Hour), legalManifest(tc.at, legalEntry("tos", "1.0.1", legalT0.Add(-10*legalDay), "v1.0.1")))
			assert.Equal(t, "2.0.0", legalVersion(legalCatalog(t, r).Current("tos", legalT0.Add(2*time.Hour))))
			got, ok := archived(t, r)["1.0.1"]
			if assert.True(t, ok, "1.0.1 is kept") {
				assert.Equal(t, "v1.0.1", got.Title["en"])
			}
		})
	}
}

func testLegalStaleUpcomingThenRepublish(t *testing.T, nc Factory) {
	now := legalT0
	p1 := legalManifest(legalT0.Add(-20*time.Minute), legalEntry("tos", "2.0.0", legalT0.Add(200*legalDay), "v2"))
	p2 := legalManifest(legalT0.Add(-10*time.Minute), legalEntry("tos", "1.0.0", legalT0.Add(-30*legalDay), "v1"))
	p3 := legalManifest(legalT0.Add(-5*time.Minute), legalEntry("tos", "2.0.0", legalT0.Add(200*legalDay), "v2"))
	var states []string
	for _, order := range [][]legal.Manifest{{p1, p2}, {p2, p1}} {
		c, _, done := nc(t)
		applyLegal(t, c.PolicyVersion, now, legalBase)
		applyLegal(t, c.PolicyVersion, now, order...)
		applyLegal(t, c.PolicyVersion, now, p3)
		states = append(states, legalState(t, c.PolicyVersion, now, "1.0.0", "2.0.0"))
		done()
	}
	assert.Equal(t, states[0], states[1], "both delivery orders, then the republish, end in the same state")
}

func testLegalTieInEffectLoser(t *testing.T, nc Factory) {
	now := legalT0.Add(time.Hour)
	versions := []string{"1.0.0", "1.0.5", "1.1.0", "1.5.0", "1.9.0", "2.0.0"}
	for _, tc := range []struct {
		name string
		base []legal.Manifest
		a, b legal.Entry
	}{
		{"empty store, in effect vs upcoming", nil,
			legalEntry("tos", "1.0.0", legalT0.Add(-30*legalDay), "v1"), legalEntry("tos", "2.0.0", legalT0.Add(30*legalDay), "v2")},
		{"base at the same timestamp, in effect vs upcoming",
			[]legal.Manifest{legalManifest(legalT0, legalEntry("tos", "1.0.0", legalT0.Add(-300*legalDay), "v1"))},
			legalEntry("tos", "1.0.5", legalT0.Add(-legalDay), "v1.0.5"), legalEntry("tos", "1.1.0", legalT0.Add(legalDay), "v1.1")},
		{"future 2.0.0 vs in-effect 1.5.0", []legal.Manifest{legalBase},
			legalEntry("tos", "2.0.0", legalT0.Add(30*legalDay), "v2"), legalEntry("tos", "1.5.0", legalT0.Add(-legalDay), "v1.5")},
		{"in-effect 2.0.0 vs in-effect 1.9.0 with a later date", []legal.Manifest{legalBase},
			legalEntry("tos", "2.0.0", legalT0.Add(-5*legalDay), "v2"), legalEntry("tos", "1.9.0", legalT0.Add(-2*legalDay), "v1.9")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, b := bothOrders(t, nc, now, now, tc.base, legalManifest(legalT0, tc.a), legalManifest(legalT0, tc.b), versions...)
			assert.Equal(t, a, b)
			assert.NotContains(t, a, "current=-")
		})
	}
}

func testLegalTieBothUpcoming(t *testing.T, nc Factory) {
	a, b := bothOrders(t, nc, legalT0, legalT0, []legal.Manifest{legalBase},
		legalManifest(legalT0, legalEntry("tos", "2.0.0", legalT0.Add(30*legalDay), "v2")),
		legalManifest(legalT0, legalEntry("tos", "3.0.0", legalT0.Add(30*legalDay), "v3")),
		"1.0.0", "2.0.0", "3.0.0")
	assert.Equal(t, a, b)
	assert.Contains(t, a, "withdrawn=[2.0.0]")
}

func testLegalArchiveKeepsWithdrawnContent(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	f := &faultyRepo{VersionRepo: c.PolicyVersion}
	now := legalT0.Add(time.Hour)
	applyLegal(t, f, now, legalBase)
	f.fail = 1
	_, err := legal.Apply(ctx, f, legalManifest(legalT0, legalEntry("tos", "2.0.0", legalT0.Add(30*legalDay), "v2")), clockAt(now))
	require.Error(t, err)
	revert := legalManifest(legalT0.Add(time.Minute), legalEntry("tos", "1.0.0", legalT0.Add(-30*legalDay), "v1"))
	applyLegal(t, f, now, revert, revert, legalBase)

	got, ok := archived(t, f)["2.0.0"]
	if assert.True(t, ok, "2.0.0 was upcoming, so its content is archived") {
		assert.Equal(t, "v2", got.Title["en"])
		assert.True(t, got.EffectiveAt.Equal(legalT0.Add(30*legalDay)))
	}
}

func testLegalConcurrentMatchesSequential(t *testing.T, nc Factory) {
	const n = 20
	now := legalT0
	rng := rand.New(rand.NewSource(1000))
	ks := rng.Perm(n)
	var payloads []legal.Manifest
	var versions []string
	for i, k := range ks {
		v := fmt.Sprintf("3.%d.0", k)
		versions = append(versions, v)
		e := legalEntry("tos", v, legalT0.Add(time.Duration(30+k)*legalDay), fmt.Sprintf("%s from payload %d", v, i))
		e.Required = k%2 == 0
		payloads = append(payloads, legalManifest(legalT0.Add(time.Duration(i-n)*9*time.Second), e))
	}
	slices.Sort(versions)
	versions = append(versions, "1.0.0")

	seq, _, doneSeq := nc(t)
	defer doneSeq()
	applyLegal(t, seq.PolicyVersion, now, legalBase)
	applyLegal(t, seq.PolicyVersion, now, payloads...)
	want := legalState(t, seq.PolicyVersion, now, versions...)
	seqCat := legalCatalog(t, seq.PolicyVersion)
	withdrawn := 0
	for _, v := range versions {
		if seqCat.Withdrawn(legal.Ref{Type: "tos", Version: legal.MustParseVersion(v)}) {
			withdrawn++
		}
	}
	require.Equal(t, n-1, withdrawn, "the oracle withdraws every version but the newest")

	conc, _, doneConc := nc(t)
	defer doneConc()
	r := conc.PolicyVersion
	applyLegal(t, r, now, legalBase)
	order := slices.Clone(payloads)
	rng.Shuffle(n, func(i, j int) { order[i], order[j] = order[j], order[i] })
	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, m := range order {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := legal.Apply(context.Background(), r, m, clockAt(now))
			assert.NoError(t, err)
		}()
	}
	close(start)
	wg.Wait()
	assert.Equal(t, want, legalState(t, r, now, versions...))

	newest := payloads[n-1].Entries[0]
	got := archived(t, r)[newest.Version.String()]
	assert.Equal(t, newest.Title, got.Title)
	assert.True(t, newest.EffectiveAt.Equal(got.EffectiveAt))
}

// A newest payload that names a withdrawn version is ignored, so the two
// delivery orders end differently. Changing legal.renamedWithdrawn changes
// these expectations.
func testLegalRenamedWithdrawn(t *testing.T, nc Factory) {
	now := legalT0
	p1 := legalManifest(legalT0.Add(time.Minute), legalEntry("tos", "3.0.0", legalT0.Add(40*legalDay), "v3"))
	p2 := legalManifest(legalT0.Add(2*time.Minute), legalEntry("tos", "2.0.0", legalT0.Add(30*legalDay), "v2"))
	base := []legal.Manifest{legalBase, legalManifest(legalT0, legalEntry("tos", "2.0.0", legalT0.Add(30*legalDay), "v2"))}
	inOrder, reversed := bothOrders(t, nc, now, legalT0.Add(31*legalDay), base, p1, p2, "2.0.0", "3.0.0")
	assert.Equal(t, `current=1.0.0@`+legalT0.Add(-30*legalDay).Format(time.RFC3339)+` "v1" upcoming=- withdrawn=[2.0.0 3.0.0]`, inOrder)
	assert.Equal(t, `current=2.0.0@`+legalT0.Add(30*legalDay).Format(time.RFC3339)+` "v2" upcoming=- withdrawn=[3.0.0]`, reversed)
}
