package conformance

import (
	"testing"
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/legal"
	"github.com/stretchr/testify/assert"
)

func runLegalTimeline(t *testing.T, nc Factory) {
	t.Run("Legal_DelayedOlderPayloadAfterItsDate", func(t *testing.T) { testLegalDelayedOlderPayload(t, nc) })
	t.Run("Legal_PlannedJustBeforeTheDate", func(t *testing.T) { testLegalPlannedBeforeDate(t, nc) })
	t.Run("Legal_TieSameVersion", func(t *testing.T) { testLegalTieSameVersion(t, nc) })
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
	}{
		{"both in effect", legalT0.Add(-legalDay), legalT0.Add(time.Hour), legalT0.Add(time.Hour)},
		{"second arrives after the date", legalT0.Add(legalDay), legalT0.Add(time.Hour), legalT0.Add(legalDay + time.Hour)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := legalManifest(legalT0, legalEntry("tos", "1.0.1", tc.eff, "A"))
			b := legalManifest(legalT0, legalEntry("tos", "1.0.1", tc.eff, "B"))
			var states []string
			for _, order := range [][]legal.Manifest{{a, b}, {b, a}} {
				c, _, done := nc(t)
				applyLegal(t, c.PolicyVersion, tc.first, legalBase, order[0])
				applyLegal(t, c.PolicyVersion, tc.then, order[1])
				states = append(states, legalState(t, c.PolicyVersion, legalT0.Add(2*legalDay), "1.0.0", "1.0.1"))
				done()
			}
			assert.Equal(t, states[0], states[1], "the same-timestamp tie winner sets the content in either order")
		})
	}
}
