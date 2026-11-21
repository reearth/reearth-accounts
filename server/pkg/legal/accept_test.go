package legal_test

import (
	"context"
	"testing"
	"time"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/legal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	day  = 24 * time.Hour
	base = time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	user = id.NewUserID()
)

type delivery struct {
	at      time.Time
	entries []legal.Entry
}

func entry(typ, version string, eff time.Time, required bool) legal.Entry {
	return legal.Entry{
		Ref:         ref(typ, version),
		EffectiveAt: eff,
		Required:    required,
		Title:       legal.LocalizedText{"en": typ},
	}
}

func ref(typ, version string) legal.Ref {
	return legal.Ref{Type: legal.Type(typ), Version: legal.MustParseVersion(version)}
}

func catalog(t *testing.T, ds ...delivery) legal.Catalog {
	t.Helper()
	ctx := context.Background()
	r := memory.NewPolicyVersion()
	for _, d := range ds {
		_, err := legal.Apply(ctx, r, legal.Manifest{Timestamp: d.at, Entries: d.entries}, func() time.Time { return d.at })
		require.NoError(t, err)
	}
	heads, err := r.FindHeads(ctx)
	require.NoError(t, err)
	return legal.NewCatalog(heads)
}

func accepted(refs ...legal.Ref) []legal.Acceptance {
	as := make([]legal.Acceptance, len(refs))
	for i, r := range refs {
		as[i] = legal.Acceptance{UserID: user, Ref: r, AcceptedAt: base}
	}
	return as
}

func pendingRefs(c legal.Catalog, as []legal.Acceptance, now time.Time) []legal.Ref {
	res := []legal.Ref{}
	for _, v := range legal.Pending(c, as, now) {
		res = append(res, v.Ref)
	}
	return res
}

func TestPending(t *testing.T) {
	tos1 := delivery{at: base, entries: []legal.Entry{entry("tos", "1.0.0", base.Add(-day), true)}}
	tos2Upcoming := delivery{at: base.Add(time.Hour), entries: []legal.Entry{entry("tos", "2.0.0", base.Add(10*day), true)}}
	tos2Reverted := delivery{at: base.Add(2 * time.Hour), entries: []legal.Entry{entry("tos", "1.0.0", base.Add(-day), true)}}
	now := base.Add(2 * day)
	afterTos2 := base.Add(11 * day)

	tests := []struct {
		name     string
		catalog  []delivery
		accepted []legal.Ref
		now      time.Time
		want     []legal.Ref
	}{
		{name: "nothing accepted", catalog: []delivery{tos1}, now: now, want: []legal.Ref{ref("tos", "1.0.0")}},
		{name: "current accepted", catalog: []delivery{tos1}, accepted: []legal.Ref{ref("tos", "1.0.0")}, now: now, want: []legal.Ref{}},
		{
			name:     "patch bump is not pending",
			catalog:  []delivery{tos1, {at: base.Add(time.Hour), entries: []legal.Entry{entry("tos", "1.0.1", base.Add(day), true)}}},
			accepted: []legal.Ref{ref("tos", "1.0.0")},
			now:      now,
			want:     []legal.Ref{},
		},
		{
			name:     "minor bump is not pending",
			catalog:  []delivery{tos1, {at: base.Add(time.Hour), entries: []legal.Entry{entry("tos", "1.1.0", base.Add(day), true)}}},
			accepted: []legal.Ref{ref("tos", "1.0.0")},
			now:      now,
			want:     []legal.Ref{},
		},
		{
			name:     "major bump is pending",
			catalog:  []delivery{tos1, tos2Upcoming},
			accepted: []legal.Ref{ref("tos", "1.0.0")},
			now:      afterTos2,
			want:     []legal.Ref{ref("tos", "2.0.0")},
		},
		{
			name:     "a major bump not yet in effect is not pending",
			catalog:  []delivery{tos1, tos2Upcoming},
			accepted: []legal.Ref{ref("tos", "1.0.0")},
			now:      now,
			want:     []legal.Ref{},
		},
		{
			name:     "accepting the upcoming version counts for the current one",
			catalog:  []delivery{tos1, tos2Upcoming},
			accepted: []legal.Ref{ref("tos", "2.0.0")},
			now:      now,
			want:     []legal.Ref{},
		},
		{
			name:     "the accepted upcoming version stays accepted once in effect",
			catalog:  []delivery{tos1, tos2Upcoming},
			accepted: []legal.Ref{ref("tos", "2.0.0")},
			now:      afterTos2,
			want:     []legal.Ref{},
		},
		{
			name:     "a withdrawn acceptance counts for nothing",
			catalog:  []delivery{tos1, tos2Upcoming, tos2Reverted},
			accepted: []legal.Ref{ref("tos", "2.0.0")},
			now:      now,
			want:     []legal.Ref{ref("tos", "1.0.0")},
		},
		{
			name:     "a withdrawn acceptance does not hide an older one",
			catalog:  []delivery{tos1, tos2Upcoming, tos2Reverted},
			accepted: []legal.Ref{ref("tos", "1.0.0"), ref("tos", "2.0.0")},
			now:      now,
			want:     []legal.Ref{},
		},
		{
			name:    "no current version",
			catalog: []delivery{{at: base, entries: []legal.Entry{entry("tos", "1.0.0", base.Add(10*day), true)}}},
			now:     now,
			want:    []legal.Ref{},
		},
		{
			name:    "not required",
			catalog: []delivery{{at: base, entries: []legal.Entry{entry("cookies", "1.0.0", base.Add(-day), false)}}},
			now:     now,
			want:    []legal.Ref{},
		},
		{
			name: "sorted by type, each type on its own",
			catalog: []delivery{{at: base, entries: []legal.Entry{
				entry("privacy", "1.0.0", base.Add(-day), true),
				entry("tos", "3.0.0", base.Add(-day), true),
			}}},
			now:  now,
			want: []legal.Ref{ref("privacy", "1.0.0"), ref("tos", "3.0.0")},
		},
		{
			name: "an acceptance of one type does not count for another",
			catalog: []delivery{{at: base, entries: []legal.Entry{
				entry("privacy", "1.0.0", base.Add(-day), true),
				entry("tos", "1.0.0", base.Add(-day), true),
			}}},
			accepted: []legal.Ref{ref("privacy", "1.0.0")},
			now:      now,
			want:     []legal.Ref{ref("tos", "1.0.0")},
		},
		{name: "empty catalog", now: now, want: []legal.Ref{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, pendingRefs(catalog(t, tt.catalog...), accepted(tt.accepted...), tt.now))
		})
	}
}

func TestPlanAccept(t *testing.T) {
	cat := catalog(t,
		delivery{at: base, entries: []legal.Entry{
			entry("privacy", "1.0.0", base.Add(-day), true),
			entry("tos", "1.0.0", base.Add(-day), true),
		}},
		delivery{at: base.Add(time.Hour), entries: []legal.Entry{entry("tos", "1.1.0", base.Add(-day/2), true)}},
		delivery{at: base.Add(2 * time.Hour), entries: []legal.Entry{entry("tos", "2.0.0", base.Add(10*day), true)}},
	)
	now := base.Add(time.Hour)

	tests := []struct {
		name     string
		accepted []legal.Ref
		req      []legal.Ref
		want     []legal.Ref
		err      error
	}{
		{name: "current", req: []legal.Ref{ref("tos", "1.1.0")}, want: []legal.Ref{ref("tos", "1.1.0")}},
		{name: "upcoming", req: []legal.Ref{ref("tos", "2.0.0")}, want: []legal.Ref{ref("tos", "2.0.0")}},
		{name: "unknown type", req: []legal.Ref{ref("tos", "1.1.0"), ref("cookies", "1.0.0")}, err: legal.ErrUnknownType},
		{name: "stale version", req: []legal.Ref{ref("tos", "1.0.0")}, err: legal.ErrNotAcceptable},
		{name: "never published", req: []legal.Ref{ref("tos", "0.9.0")}, err: legal.ErrNotAcceptable},
		{name: "one bad ref rejects the request", req: []legal.Ref{ref("privacy", "1.0.0"), ref("tos", "9.0.0")}, err: legal.ErrNotAcceptable},
		{name: "repeat writes nothing", accepted: []legal.Ref{ref("tos", "1.1.0")}, req: []legal.Ref{ref("tos", "1.1.0")}, want: []legal.Ref{}},
		{name: "lower than held writes nothing", accepted: []legal.Ref{ref("tos", "2.0.0")}, req: []legal.Ref{ref("tos", "1.1.0")}, want: []legal.Ref{}},
		{name: "higher than held", accepted: []legal.Ref{ref("tos", "1.0.0")}, req: []legal.Ref{ref("tos", "1.1.0")}, want: []legal.Ref{ref("tos", "1.1.0")}},
		{name: "held stale version writes nothing", accepted: []legal.Ref{ref("tos", "1.0.0")}, req: []legal.Ref{ref("tos", "1.0.0")}, want: []legal.Ref{}},
		{name: "unpublished version below held writes nothing", accepted: []legal.Ref{ref("tos", "1.1.0")}, req: []legal.Ref{ref("tos", "0.9.0")}, want: []legal.Ref{}},
		{
			name: "every requested version above held, sorted",
			req:  []legal.Ref{ref("tos", "2.0.0"), ref("privacy", "1.0.0"), ref("tos", "1.1.0"), ref("tos", "2.0.0")},
			want: []legal.Ref{ref("privacy", "1.0.0"), ref("tos", "1.1.0"), ref("tos", "2.0.0")},
		},
		{name: "empty request", want: []legal.Ref{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := legal.PlanAccept(cat, accepted(tt.accepted...), tt.req, now)
			if tt.err != nil {
				assert.ErrorIs(t, err, tt.err)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPlanAccept_BothVersionsSurviveWithdrawingTheUpcoming(t *testing.T) {
	first := delivery{at: base, entries: []legal.Entry{entry("tos", "1.0.0", base.Add(-day), true)}}
	upcoming := delivery{at: base.Add(time.Hour), entries: []legal.Entry{entry("tos", "2.0.0", base.Add(10*day), true)}}
	now := base.Add(time.Hour)
	plan, err := legal.PlanAccept(catalog(t, first, upcoming), nil, []legal.Ref{ref("tos", "1.0.0"), ref("tos", "2.0.0")}, now)
	require.NoError(t, err)

	reverted := delivery{at: base.Add(2 * time.Hour), entries: []legal.Entry{entry("tos", "1.0.0", base.Add(-day), true)}}
	assert.Empty(t, pendingRefs(catalog(t, first, upcoming, reverted), accepted(plan...), now))
}

func TestParseRefs(t *testing.T) {
	got, err := legal.ParseRefs([]legal.RefInput{{Type: "tos", Version: "1.2.3"}})
	require.NoError(t, err)
	assert.Equal(t, []legal.Ref{ref("tos", "1.2.3")}, got)

	_, err = legal.ParseRefs([]legal.RefInput{{Type: "tos", Version: "1.2"}})
	assert.ErrorIs(t, err, legal.ErrInvalidVersion)
	assert.EqualError(t, err, "version must be MAJOR.MINOR.PATCH")
	_, err = legal.ParseRefs([]legal.RefInput{{Type: "TOS!", Version: "1.0.0"}})
	assert.ErrorIs(t, err, legal.ErrUnknownType)
}

func TestPlanAccept_WithdrawnIsNotAcceptable(t *testing.T) {
	cat := catalog(t,
		delivery{at: base, entries: []legal.Entry{entry("tos", "2.0.0", base.Add(10*day), true)}},
		delivery{at: base.Add(time.Hour), entries: []legal.Entry{entry("tos", "1.0.0", base.Add(-day), true)}},
	)
	_, err := legal.PlanAccept(cat, nil, []legal.Ref{ref("tos", "2.0.0")}, base.Add(time.Hour))
	assert.ErrorIs(t, err, legal.ErrNotAcceptable)
}

func TestBuildOverview(t *testing.T) {
	cat := catalog(t,
		delivery{at: base, entries: []legal.Entry{
			entry("privacy", "1.0.0", base.Add(-day), true),
			entry("tos", "1.0.0", base.Add(-day), true),
		}},
		delivery{at: base.Add(time.Hour), entries: []legal.Entry{entry("tos", "2.0.0", base.Add(10*day), true)}},
		delivery{at: base.Add(2 * time.Hour), entries: []legal.Entry{entry("privacy", "1.1.0", base.Add(5*day), false)}},
	)
	as := accepted(ref("tos", "1.0.0"), ref("tos", "2.0.0"), ref("gone", "1.0.0"))

	ov := legal.BuildOverview(cat, as, base.Add(2*time.Hour))

	held := []legal.Ref{}
	for _, a := range ov.Acceptances {
		held = append(held, a.Ref)
	}
	assert.Equal(t, []legal.Ref{ref("gone", "1.0.0"), ref("tos", "2.0.0")}, held)
	assert.Equal(t, []legal.Ref{ref("privacy", "1.0.0")}, refsOf(ov.Pending))
	assert.Equal(t, []legal.Ref{ref("privacy", "1.1.0"), ref("tos", "2.0.0")}, refsOf(ov.Upcoming))
}

func TestEmptyOverview(t *testing.T) {
	ov := legal.EmptyOverview()
	assert.NotNil(t, ov.Acceptances)
	assert.NotNil(t, ov.Pending)
	assert.NotNil(t, ov.Upcoming)
}

func TestLocalizedTextPick(t *testing.T) {
	text := legal.LocalizedText{"en": "Terms", "ja": "規約", "de": "AGB"}
	assert.Equal(t, "規約", text.Pick("ja"))
	assert.Equal(t, "規約", text.Pick("ja-JP"))
	assert.Equal(t, "Terms", text.Pick("fr"))
	assert.Equal(t, "Terms", text.Pick(""))
	assert.Equal(t, "AGB", legal.LocalizedText{"fr": "CGU", "de": "AGB"}.Pick("es"))
	assert.Equal(t, "", legal.LocalizedText(nil).Pick("en"))
}

func refsOf(vs []legal.VersionRecord) []legal.Ref {
	res := []legal.Ref{}
	for _, v := range vs {
		res = append(res, v.Ref)
	}
	return res
}
