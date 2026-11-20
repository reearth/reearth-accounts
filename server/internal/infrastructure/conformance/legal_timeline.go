package conformance

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/legal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runLegalTimeline(t *testing.T, nc Factory) {
	t.Run("Legal_DelayedOlderPayloadAfterItsDate", func(t *testing.T) { testLegalDelayedOlderPayload(t, nc) })
	t.Run("Legal_PlannedJustBeforeTheDate", func(t *testing.T) { testLegalPlannedBeforeDate(t, nc) })
	t.Run("Legal_TieSameVersion", func(t *testing.T) { testLegalTieSameVersion(t, nc) })
	t.Run("Legal_OlderPayloadAfterTwoNewer", func(t *testing.T) { testLegalOlderAfterTwoNewer(t, nc) })
	t.Run("Legal_OlderPayloadAfterFutureStampedRevert", func(t *testing.T) { testLegalOlderAfterFutureStampedRevert(t, nc) })
	t.Run("Legal_RepublishWithdrawnWithPastDate", func(t *testing.T) { testLegalRepublishWithdrawnPastDate(t, nc) })
	t.Run("Legal_DelayedFirstPayloadThenRevert", func(t *testing.T) { testLegalDelayedFirstThenRevert(t, nc) })
	t.Run("Legal_OlderPayloadAddsNoType", func(t *testing.T) { testLegalOlderPayloadAddsNoType(t, nc) })
}

var legalTimelineVersions = []string{"1.0.0", "2.0.0", "3.0.0"}

func testLegalDelayedOlderPayload(t *testing.T, nc Factory) {
	older := legalManifest(legalT0, legalEntry("tos", "2.0.0", legalT0.Add(legalDay), "v2"))
	readAt := legalT0.Add(2 * legalDay)
	for _, tc := range []struct {
		name  string
		newer legal.Manifest
		want  string
	}{
		{
			"newer publishes 3.0.0",
			legalManifest(legalT0.Add(time.Minute), legalEntry("tos", "3.0.0", legalT0.Add(30*legalDay), "v3")),
			`current=1.0.0@2026-10-21T03:00:00Z "v1" upcoming=3.0.0@2026-12-20T03:00:00Z "v3" withdrawn=[2.0.0]`,
		},
		{
			"newer reverts to 1.0.0",
			legalManifest(legalT0.Add(time.Minute), legalEntry("tos", "1.0.0", legalT0.Add(-30*legalDay), "v1")),
			`current=1.0.0@2026-10-21T03:00:00Z "v1" upcoming=- withdrawn=[2.0.0]`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, d := range []struct {
				name      string
				olderAt   time.Time
				olderLast bool
			}{
				{"in order", legalT0.Add(time.Hour), false},
				{"older late, before its date", legalT0.Add(time.Hour), true},
				{"older late, after its date", legalT0.Add(legalDay + time.Hour), true},
			} {
				c, _, done := nc(t)
				r := c.PolicyVersion
				applyLegal(t, r, legalT0.Add(time.Hour), legalBase)
				if !d.olderLast {
					applyLegal(t, r, d.olderAt, older)
				}
				applyLegal(t, r, legalT0.Add(time.Hour), tc.newer)
				if d.olderLast {
					applyLegal(t, r, d.olderAt, older)
				}
				assert.Equal(t, tc.want, legalState(t, r, readAt, legalTimelineVersions...), d.name)
				done()
			}
		})
	}
}

func testLegalPlannedBeforeDate(t *testing.T, nc Factory) {
	eff := legalT0.Add(legalDay)
	shown := `current=2.0.0@2026-11-21T03:00:00Z "v2" upcoming=- withdrawn=[]`
	revert := legalManifest(legalT0.Add(time.Minute), legalEntry("tos", "1.0.0", legalT0.Add(-30*legalDay), "v1"))
	correction := legalManifest(legalT0.Add(time.Minute), legalEntry("tos", "2.0.0", eff.Add(10*legalDay), "v2-corrected"))
	for _, tc := range []struct {
		name    string
		payload legal.Manifest
		planAt  time.Time
		want    string
	}{
		{"revert planned 100ms before the date", revert, eff.Add(-100 * time.Millisecond), shown},
		{"correction planned 100ms before the date", correction, eff.Add(-100 * time.Millisecond), shown},
		{"correction on a clock 2s behind, 1s after the date", correction, eff.Add(-time.Second), shown},
		{"revert just inside the freeze margin", revert, eff.Add(-legal.FreezeMargin), shown},
		{"revert before the freeze margin", revert, eff.Add(-legal.FreezeMargin - time.Millisecond), `current=1.0.0@2026-10-21T03:00:00Z "v1" upcoming=- withdrawn=[2.0.0]`},
		{"correction before the freeze margin", correction, eff.Add(-2 * legal.FreezeMargin), `current=1.0.0@2026-10-21T03:00:00Z "v1" upcoming=2.0.0@2026-12-01T03:00:00Z "v2-corrected" withdrawn=[]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _, done := nc(t)
			defer done()
			r := c.PolicyVersion
			applyLegal(t, r, legalT0, legalBase, legalManifest(legalT0, legalEntry("tos", "2.0.0", eff, "v2")))
			applyLegal(t, r, tc.planAt, tc.payload)
			assert.Equal(t, tc.want, legalState(t, r, eff.Add(2*time.Second), legalTimelineVersions...))
		})
	}
}

func testLegalTieSameVersion(t *testing.T, nc Factory) {
	for _, tc := range []struct {
		name             string
		eff, first, then time.Time
		converges        bool
	}{
		{"both in effect", legalT0.Add(-legalDay), legalT0.Add(time.Hour), legalT0.Add(time.Hour), false},
		{"second arrives after the date", legalT0.Add(legalDay), legalT0.Add(time.Hour), legalT0.Add(legalDay + time.Hour), false},
		{"both before the date", legalT0.Add(legalDay), legalT0.Add(time.Hour), legalT0.Add(2 * time.Hour), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := legalManifest(legalT0, legalEntry("tos", "1.0.1", tc.eff, "A"))
			b := legalManifest(legalT0, legalEntry("tos", "1.0.1", tc.eff, "B"))
			var states []string
			for _, order := range [][]legal.Manifest{{a, b}, {b, a}} {
				c, _, done := nc(t)
				applyLegal(t, c.PolicyVersion, tc.first, legalBase, order[0])
				applyLegal(t, c.PolicyVersion, tc.then, order[1])
				state := legalState(t, c.PolicyVersion, legalT0.Add(2*legalDay), "1.0.0", "1.0.1")
				if !tc.converges {
					assert.Contains(t, state, fmt.Sprintf("%q", order[0].Entries[0].Title["en"]), "a version in effect keeps its first content")
				}
				states = append(states, state)
				done()
			}
			if tc.converges {
				assert.Equal(t, states[0], states[1], "the same-timestamp tie winner sets the content in either order")
			}
		})
	}
}

type delivery struct {
	at time.Time
	m  legal.Manifest
}

func deliver(t *testing.T, nc Factory, readAt time.Time, ds ...delivery) string {
	t.Helper()
	c, _, done := nc(t)
	defer done()
	for _, d := range ds {
		applyLegal(t, c.PolicyVersion, d.at, d.m)
	}
	return legalState(t, c.PolicyVersion, readAt, legalTimelineVersions...)
}

func testLegalOlderAfterTwoNewer(t *testing.T, nc Factory) {
	day := func(d int) time.Time { return legalT0.Add(time.Duration(d) * legalDay) }
	base := delivery{day(-3), legalManifest(day(-3), legalEntry("tos", "1.0.0", day(-30), "v1"))}
	p1 := legalManifest(day(-2), legalEntry("tos", "2.0.0", day(0), "v2"))
	p2 := legalManifest(day(-1), legalEntry("tos", "1.0.0", day(-30), "v1"))
	p3 := legalManifest(day(1), legalEntry("tos", "3.0.0", day(30), "v3"))
	want := `current=1.0.0@2026-10-21T03:00:00Z "v1" upcoming=3.0.0@2026-12-20T03:00:00Z "v3" withdrawn=[2.0.0]`
	assert.Equal(t, want, deliver(t, nc, day(3), base, delivery{day(-2), p1}, delivery{day(-1), p2}, delivery{day(1), p3}), "in order")
	assert.Equal(t, want, deliver(t, nc, day(3), base, delivery{day(-1), p2}, delivery{day(1), p3}, delivery{day(2), p1}), "the oldest retried last, after its date")
}

func testLegalOlderAfterFutureStampedRevert(t *testing.T, nc Factory) {
	base := delivery{legalT0, legalBase}
	p1 := delivery{legalT0, legalManifest(legalT0.Add(-time.Hour), legalEntry("tos", "2.0.0", legalT0.Add(3*time.Minute), "v2"))}
	p2 := delivery{legalT0, legalManifest(legalT0.Add(4*time.Minute), legalEntry("tos", "1.0.0", legalT0.Add(-30*legalDay), "v1"))}
	want := `current=1.0.0@2026-10-21T03:00:00Z "v1" upcoming=- withdrawn=[2.0.0]`
	readAt := legalT0.Add(time.Hour)
	assert.Equal(t, want, deliver(t, nc, readAt, base, p1, p2), "in order")
	assert.Equal(t, want, deliver(t, nc, readAt, base, p2, p1), "the revert stamped ahead of the receiver first")
}

// everyOrder delivers base and then ps in every order, each payload at its
// own timestamp or at the previous delivery's time if that is later, and
// returns the state read at readAt by order.
func everyOrder(t *testing.T, nc Factory, readAt time.Time, base []legal.Manifest, ps ...legal.Manifest) map[string]string {
	t.Helper()
	out := map[string]string{}
	var walk func(order []int, rest []int)
	walk = func(order []int, rest []int) {
		if len(rest) == 0 {
			var ds []delivery
			var at time.Time
			for _, m := range base {
				at = maxTime(at, m.Timestamp)
				ds = append(ds, delivery{at, m})
			}
			for _, i := range order {
				at = maxTime(at, ps[i].Timestamp)
				ds = append(ds, delivery{at, ps[i]})
			}
			out[fmt.Sprint(order)] = deliver(t, nc, readAt, ds...)
			return
		}
		for j, i := range rest {
			walk(append(slices.Clone(order), i), slices.Delete(slices.Clone(rest), j, j+1))
		}
	}
	all := make([]int, len(ps))
	for i := range all {
		all[i] = i
	}
	walk(nil, all)
	return out
}

func maxTime(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

func testLegalRepublishWithdrawnPastDate(t *testing.T, nc Factory) {
	h := time.Hour
	base := legalManifest(legalT0.Add(-3*h), legalEntry("tos", "1.0.0", legalT0.Add(-30*legalDay), "v1"))
	p1 := legalManifest(legalT0.Add(-2*h), legalEntry("tos", "2.0.0", legalT0.Add(h), "v2 planned"))
	p2 := legalManifest(legalT0.Add(-h), legalEntry("tos", "1.0.0", legalT0.Add(-30*legalDay), "v1"))
	p3 := legalManifest(legalT0, legalEntry("tos", "2.0.0", legalT0, "v2 now"))
	want := `current=2.0.0@2026-11-20T03:00:00Z "v2 now" upcoming=- withdrawn=[]`
	for order, got := range everyOrder(t, nc, legalT0.Add(2*h), []legal.Manifest{base}, p1, p2, p3) {
		assert.Equal(t, want, got, "the newest payload republishes 2.0.0 in effect, order %s", order)
	}
}

func testLegalDelayedFirstThenRevert(t *testing.T, nc Factory) {
	h := time.Hour
	p0 := legalManifest(legalT0.Add(-2*h), legalEntry("tos", "1.0.0", legalT0.Add(-30*legalDay), "v1"))
	p1 := legalManifest(legalT0.Add(-h), legalEntry("tos", "2.0.0", legalT0.Add(30*legalDay), "v2"))
	p2 := legalManifest(legalT0, legalEntry("tos", "1.0.0", legalT0.Add(-30*legalDay), "v1"))
	want := `current=1.0.0@2026-10-21T03:00:00Z "v1" upcoming=- withdrawn=[2.0.0]`
	for order, got := range everyOrder(t, nc, legalT0.Add(h), nil, p0, p1, p2) {
		assert.Equal(t, want, got, "the revert keeps 1.0.0 current, order %s", order)
	}
}

func testLegalOlderPayloadAddsNoType(t *testing.T, nc Factory) {
	h := time.Hour
	past := legalT0.Add(-2 * legalDay)
	p0 := legalManifest(legalT0.Add(-3*h), legalEntry("cookie", "1.0.0", past, "c1"), legalEntry("tos", "1.0.0", past, "v1"))
	p2 := legalManifest(legalT0.Add(-h), legalEntry("tos", "1.0.0", past, "v1"))
	p3 := legalManifest(legalT0, legalEntry("tos", "1.0.0", past, "v1"))

	c, _, done := nc(t)
	defer done()
	r := c.PolicyVersion
	applyLegal(t, r, legalT0, p2)
	res, err := legal.Apply(context.Background(), r, p0, clockAt(legalT0))
	require.NoError(t, err)
	assert.Equal(t, []legal.Type{"cookie", "tos"}, res.Ignored)
	applyLegal(t, r, legalT0, p3)
	cat := legalCatalog(t, r)
	assert.Equal(t, []legal.Type{"tos"}, cat.Types(), "an older payload adds no type")
	assert.Nil(t, cat.Current("cookie", legalT0))
	recs, err := r.FindVersions(context.Background())
	require.NoError(t, err)
	for _, rec := range recs {
		assert.NotEqual(t, legal.Type("cookie"), rec.Type, "nothing of the older payload is archived")
	}

	sameTime := legalManifest(legalT0, legalEntry("cookie", "1.0.0", past, "c1"))
	newer := legalManifest(legalT0.Add(h), legalEntry("privacy", "1.0.0", past, "p1"))
	applyLegal(t, r, legalT0.Add(h), sameTime, newer)
	cat = legalCatalog(t, r)
	assert.Equal(t, []legal.Type{"cookie", "privacy", "tos"}, cat.Types(), "a payload not older than the newest one adds its types")

	first, _, doneFirst := nc(t)
	defer doneFirst()
	applyLegal(t, first.PolicyVersion, legalT0, p0)
	assert.Equal(t, []legal.Type{"cookie", "tos"}, legalCatalog(t, first.PolicyVersion).Types(), "the first payload creates every type it names")
}
