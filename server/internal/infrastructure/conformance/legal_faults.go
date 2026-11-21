package conformance

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/legal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const legalDay = 24 * time.Hour

var errInjected = errors.New("injected archive failure")

// faultyRepo fails or delays archive writes, as a client hang-up or a crash
// after the head write would.
type faultyRepo struct {
	legal.VersionRepo
	mu   sync.Mutex
	fail int
	hold bool
	held []func() error
}

func (f *faultyRepo) ArchiveVersions(ctx context.Context, h legal.TypeHead) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.hold {
		f.hold = false
		f.held = append(f.held, func() error { return f.VersionRepo.ArchiveVersions(ctx, h) })
		return nil
	}
	if f.fail > 0 {
		f.fail--
		return errInjected
	}
	return f.VersionRepo.ArchiveVersions(ctx, h)
}

func legalCatalog(t *testing.T, r legal.VersionRepo) legal.Catalog {
	t.Helper()
	heads, err := r.FindHeads(context.Background())
	require.NoError(t, err)
	return legal.NewCatalog(heads)
}

func legalVersion(r *legal.VersionRecord) string {
	if r == nil {
		return ""
	}
	return r.Version.String()
}

func runLegalFaults(t *testing.T, nc Factory) {
	t.Run("Legal_ArchiveInsertRace", func(t *testing.T) { faultArchiveInsertRace(t, nc) })
	t.Run("Legal_CorrectionArchiveFails", func(t *testing.T) { faultCorrectionArchiveFails(t, nc) })
	t.Run("Legal_CrashAfterHead", func(t *testing.T) { faultCrashAfterHead(t, nc) })
	t.Run("Legal_DelayedOlderArchiveAfterRevert", func(t *testing.T) { faultDelayedOlderArchiveAfterRevert(t, nc) })
	t.Run("Legal_ArchiveFailsResendAfterEffect", func(t *testing.T) { faultResendAfterEffect(t, nc) })
	t.Run("Legal_EqualTimestampTie", func(t *testing.T) { testLegalEqualTimestampTie(t, nc) })
	t.Run("Legal_InEffectPatchThenNewerRevert", func(t *testing.T) { testLegalPatchThenRevert(t, nc) })
}

var legalBase = legalManifest(legalT0.Add(-legalDay), legalEntry("tos", "1.0.0", legalT0.Add(-30*legalDay), "v1"))

func faultArchiveInsertRace(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	r := c.PolicyVersion
	lost := 0
	const rounds = 300
	for round := range rounds {
		typ := fmt.Sprintf("tos-%d", round)
		head := func(rev int64, eff time.Duration, title string) legal.TypeHead {
			v := legalHeadVersion(typ, "2.0.0", legalT0.Add(eff), title, nil)
			v.Revision = rev
			return legal.TypeHead{Type: legal.Type(typ), Revision: rev, Applied: legalApplied, AppliedAt: legalT0, Versions: legalVersions(v)}
		}
		older, newer := head(1, 30*legalDay, "older"), head(2, 40*legalDay, "newer")
		won, err := r.CompareAndSwapHead(ctx, older)
		require.NoError(t, err)
		require.True(t, won)
		start := make(chan struct{})
		var wg sync.WaitGroup
		for _, h := range []legal.TypeHead{older, newer} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				assert.NoError(t, r.ArchiveVersions(ctx, h))
			}()
		}
		close(start)
		wg.Wait()
		recs, err := r.FindVersions(ctx)
		require.NoError(t, err)
		for _, rec := range recs {
			if string(rec.Type) == typ && rec.Title["en"] != "newer" {
				lost++
			}
		}
	}
	assert.Zero(t, lost, "rounds where the newer archive write was lost, of %d", rounds)
}

func faultCorrectionArchiveFails(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	f := &faultyRepo{VersionRepo: c.PolicyVersion}
	_, err := legal.Apply(ctx, f, legalBase, clockAt(legalT0))
	require.NoError(t, err)
	_, err = legal.Apply(ctx, f, legalManifest(legalT0, legalEntry("tos", "2.0.0", legalT0.Add(30*legalDay), "v2")), clockAt(legalT0))
	require.NoError(t, err)
	f.fail = 1
	_, _ = legal.Apply(ctx, f, legalManifest(legalT0.Add(time.Minute), legalEntry("tos", "2.0.0", legalT0.Add(40*legalDay), "v2")), clockAt(legalT0.Add(time.Minute)))

	at35 := legalT0.Add(35 * legalDay)
	cat := legalCatalog(t, f)
	assert.Equal(t, "1.0.0", legalVersion(cat.Current("tos", at35)), "the corrected date is in force")
	assert.Equal(t, "2.0.0", legalVersion(cat.Upcoming("tos", at35)))

	at31 := legalT0.Add(31 * legalDay)
	_, err = legal.Apply(ctx, f, legalManifest(at31, legalEntry("tos", "1.0.0", legalT0.Add(-30*legalDay), "v1")), clockAt(at31))
	require.NoError(t, err)
	cat = legalCatalog(t, f)
	assert.Equal(t, "1.0.0", legalVersion(cat.Current("tos", legalT0.Add(50*legalDay))), "the revert withdrew 2.0.0")
	assert.True(t, cat.Withdrawn(legal.Ref{Type: "tos", Version: legal.MustParseVersion("2.0.0")}))
}

func faultCrashAfterHead(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	f := &faultyRepo{VersionRepo: c.PolicyVersion}
	_, err := legal.Apply(ctx, f, legalBase, clockAt(legalT0))
	require.NoError(t, err)
	f.fail = 1
	_, _ = legal.Apply(ctx, f, legalManifest(legalT0, legalEntry("tos", "2.0.0", legalT0.Add(30*legalDay), "v2")), clockAt(legalT0))

	cat := legalCatalog(t, f)
	assert.Equal(t, "2.0.0", legalVersion(cat.Upcoming("tos", legalT0.Add(time.Hour))), "the applied head decides")
	assert.Equal(t, "2.0.0", legalVersion(cat.Current("tos", legalT0.Add(31*legalDay))))
}

func faultDelayedOlderArchiveAfterRevert(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	f := &faultyRepo{VersionRepo: c.PolicyVersion}
	_, err := legal.Apply(ctx, f, legalBase, clockAt(legalT0))
	require.NoError(t, err)
	f.hold = true
	_, err = legal.Apply(ctx, f, legalManifest(legalT0, legalEntry("tos", "2.0.0", legalT0.Add(30*legalDay), "v2")), clockAt(legalT0))
	require.NoError(t, err)
	f.fail = 1
	_, _ = legal.Apply(ctx, f, legalManifest(legalT0.Add(time.Minute), legalEntry("tos", "2.0.0", legalT0.Add(40*legalDay), "v2")), clockAt(legalT0.Add(time.Minute)))
	at31 := legalT0.Add(31 * legalDay)
	_, err = legal.Apply(ctx, f, legalManifest(at31, legalEntry("tos", "1.0.0", legalT0.Add(-30*legalDay), "v1")), clockAt(at31))
	require.NoError(t, err)
	for _, w := range f.held {
		require.NoError(t, w())
	}
	cat := legalCatalog(t, f)
	assert.Equal(t, "1.0.0", legalVersion(cat.Current("tos", legalT0.Add(50*legalDay))))
	assert.True(t, cat.Withdrawn(legal.Ref{Type: "tos", Version: legal.MustParseVersion("2.0.0")}))
}

func faultResendAfterEffect(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	f := &faultyRepo{VersionRepo: c.PolicyVersion}
	_, err := legal.Apply(ctx, f, legalBase, clockAt(legalT0))
	require.NoError(t, err)
	p := legalManifest(legalT0, legalEntry("tos", "2.0.0", legalT0.Add(legalDay), "v2"))
	f.fail = 1
	_, _ = legal.Apply(ctx, f, p, clockAt(legalT0))
	at2 := legalT0.Add(2 * legalDay)
	resend := legalManifest(at2, p.Entries...)
	_, err = legal.Apply(ctx, f, resend, clockAt(at2))
	require.NoError(t, err)

	assert.Equal(t, "2.0.0", legalVersion(legalCatalog(t, f).Current("tos", at2)))
	recs, err := f.FindVersions(ctx)
	require.NoError(t, err)
	found := false
	for _, rec := range recs {
		found = found || rec.Version.String() == "2.0.0"
	}
	assert.True(t, found, "the redelivery archives 2.0.0")
}

func testLegalEqualTimestampTie(t *testing.T, nc Factory) {
	cases := []struct {
		name string
		a, b legal.Entry
	}{
		{"different versions", legalEntry("tos", "2.0.0", legalT0.Add(30*legalDay), "v2"), legalEntry("tos", "3.0.0", legalT0.Add(30*legalDay), "v3")},
		{"same version, different content", legalEntry("tos", "2.0.0", legalT0.Add(30*legalDay), "draft"), legalEntry("tos", "2.0.0", legalT0.Add(40*legalDay), "final")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var views []string
			for _, order := range [][]legal.Entry{{tc.a, tc.b}, {tc.b, tc.a}} {
				c, _, done := nc(t)
				ctx := context.Background()
				_, err := legal.Apply(ctx, c.PolicyVersion, legalBase, clockAt(legalT0))
				require.NoError(t, err)
				for _, e := range order {
					_, err := legal.Apply(ctx, c.PolicyVersion, legalManifest(legalT0, e), clockAt(legalT0))
					require.NoError(t, err)
				}
				up := legalCatalog(t, c.PolicyVersion).Upcoming("tos", legalT0)
				require.NotNil(t, up)
				views = append(views, fmt.Sprintf("%s %s %s", up.Version, up.EffectiveAt.UTC().Format(time.RFC3339), up.Title["en"]))
				done()
			}
			assert.Equal(t, views[0], views[1], "both orders pick the same entry")
		})
	}
}

func testLegalPatchThenRevert(t *testing.T, nc Factory) {
	patch := legalManifest(legalT0, legalEntry("tos", "1.0.1", legalT0, "v1.0.1"))
	revert := legalManifest(legalT0.Add(time.Minute), legalEntry("tos", "1.0.0", legalT0.Add(-30*legalDay), "v1"))
	now := legalT0.Add(time.Hour)
	for _, tc := range []struct {
		name    string
		order   []legal.Manifest
		current string
	}{
		{"older first keeps the patch in effect", []legal.Manifest{patch, revert}, "1.0.1"},
		{"newer first leaves the older patch withdrawn", []legal.Manifest{revert, patch}, "1.0.0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _, done := nc(t)
			defer done()
			ctx := context.Background()
			_, err := legal.Apply(ctx, c.PolicyVersion, legalBase, clockAt(now))
			require.NoError(t, err)
			for _, m := range tc.order {
				_, err := legal.Apply(ctx, c.PolicyVersion, m, clockAt(now))
				require.NoError(t, err)
			}
			cat := legalCatalog(t, c.PolicyVersion)
			assert.Equal(t, tc.current, legalVersion(cat.Current("tos", now)))
			assert.Nil(t, cat.Upcoming("tos", now))
			assert.Equal(t, tc.current == "1.0.0", cat.Withdrawn(legal.Ref{Type: "tos", Version: legal.MustParseVersion("1.0.1")}), "withdrawn 1.0.1")
		})
	}
}
