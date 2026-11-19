package legal

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var t0 = time.Date(2026, 11, 20, 3, 0, 0, 0, time.UTC)

// fakeRepo is a sequential VersionRepo that honors the repo contract.
type fakeRepo struct {
	failArchive bool
	heads       map[Type]TypeHead
	versions    map[Ref]VersionRecord
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{heads: map[Type]TypeHead{}, versions: map[Ref]VersionRecord{}}
}

func (f *fakeRepo) ArchiveVersions(_ context.Context, h TypeHead) error {
	if f.failArchive {
		f.failArchive = false
		return errors.New("archive unavailable")
	}
	for _, v := range []*VersionRecord{h.Current, h.Upcoming} {
		if v != nil {
			f.versions[v.Ref] = *v
		}
	}
	return nil
}

func (f *fakeRepo) CompareAndSwapHead(_ context.Context, next TypeHead) (bool, error) {
	if f.heads[next.Type].Revision != next.Revision-1 {
		return false, nil
	}
	f.heads[next.Type] = next
	return true, nil
}

func (f *fakeRepo) FindHead(_ context.Context, t Type) (TypeHead, error) {
	if h, ok := f.heads[t]; ok {
		return h, nil
	}
	return TypeHead{Type: t}, nil
}

func (f *fakeRepo) FindHeads(context.Context) ([]TypeHead, error) {
	var hs []TypeHead
	for _, h := range f.heads {
		hs = append(hs, h)
	}
	return hs, nil
}

func (f *fakeRepo) FindVersions(context.Context) ([]VersionRecord, error) {
	var rs []VersionRecord
	for _, r := range f.versions {
		rs = append(rs, r)
	}
	return rs, nil
}

func (f *fakeRepo) catalog() Catalog {
	hs, _ := f.FindHeads(context.Background())
	return NewCatalog(hs)
}

type step struct {
	at      time.Duration // payload timestamp and clock, from t0
	clock   time.Duration // clock offset when it differs from at
	version string
	eff     time.Duration // effective date, from t0
	title   string
}

func entry(version string, eff time.Duration, title string) Entry {
	return Entry{
		Ref:         Ref{Type: "tos", Version: MustParseVersion(version)},
		EffectiveAt: t0.Add(eff),
		Required:    true,
		Title:       LocalizedText{"en": title},
	}
}

func run(t *testing.T, f *fakeRepo, steps []step) []ApplyResult {
	t.Helper()
	var out []ApplyResult
	for _, s := range steps {
		clock := s.at
		if s.clock != 0 {
			clock = s.clock
		}
		m := Manifest{Timestamp: t0.Add(s.at), Entries: []Entry{entry(s.version, s.eff, s.title)}}
		res, err := Apply(context.Background(), f, m, func() time.Time { return t0.Add(clock) })
		require.NoError(t, err)
		out = append(out, res)
	}
	return out
}

func versionOf(r *VersionRecord) string {
	if r == nil {
		return ""
	}
	return r.Version.String()
}

const (
	day  = 24 * time.Hour
	hour = time.Hour
)

func TestApply(t *testing.T) {
	tests := []struct {
		name          string
		steps         []step
		at            time.Duration // catalog read time
		current       string
		upcoming      string
		upcomingTitle string
		withdrawn     []string
		absent        []string
		results       []bool // Applied per step
	}{
		{
			name:     "first version already in effect becomes current",
			steps:    []step{{at: 0, version: "1.0.0", eff: -day, title: "v1"}},
			current:  "1.0.0",
			results:  []bool{true},
			upcoming: "",
		},
		{
			name:     "new version ahead of its date is upcoming and nothing is current yet",
			steps:    []step{{at: 0, version: "2.0.0", eff: 30 * day, title: "v2"}},
			upcoming: "2.0.0", upcomingTitle: "v2",
			results: []bool{true},
		},
		{
			name: "replay of the same payload changes nothing",
			steps: []step{
				{at: 0, version: "2.0.0", eff: 30 * day, title: "v2"},
				{at: 0, clock: hour, version: "2.0.0", eff: 30 * day, title: "v2"},
			},
			at:       hour,
			upcoming: "2.0.0", upcomingTitle: "v2",
			results: []bool{true, false},
		},
		{
			name: "a payload with the same timestamp and a higher version wins",
			steps: []step{
				{at: 0, version: "2.0.0", eff: 30 * day, title: "v2"},
				{at: 0, version: "1.0.0", eff: -day, title: "v1"},
			},
			upcoming: "2.0.0", upcomingTitle: "v2",
			absent:  []string{"1.0.0"},
			results: []bool{true, false},
		},
		{
			name: "older payload is ignored",
			steps: []step{
				{at: hour, version: "1.0.0", eff: -day, title: "v1"},
				{at: 0, clock: 2 * hour, version: "2.0.0", eff: 30 * day, title: "v2"},
			},
			at:      2 * hour,
			current: "1.0.0",
			absent:  []string{"2.0.0"},
			results: []bool{true, false},
		},
		{
			name: "reverting withdraws the upcoming version and it never takes effect",
			steps: []step{
				{at: 0, version: "1.0.0", eff: -day, title: "v1"},
				{at: hour, version: "2.0.0", eff: 30 * day, title: "v2"},
				{at: 2 * hour, version: "1.0.0", eff: -day, title: "v1"},
			},
			at:        60 * day,
			current:   "1.0.0",
			withdrawn: []string{"2.0.0"},
			results:   []bool{true, true, true},
		},
		{
			name: "a later payload corrects a version that has not taken effect",
			steps: []step{
				{at: 0, version: "2.0.0", eff: 30 * day, title: "draft"},
				{at: hour, version: "2.0.0", eff: 40 * day, title: "final"},
			},
			at:       35 * day,
			upcoming: "2.0.0", upcomingTitle: "final",
			results: []bool{true, true},
		},
		{
			name: "a version in effect never changes",
			steps: []step{
				{at: 0, version: "1.0.0", eff: -day, title: "v1"},
				{at: hour, version: "1.0.0", eff: 30 * day, title: "rewritten"},
			},
			at:      2 * hour,
			current: "1.0.0",
			results: []bool{true, true},
		},
		{
			name: "a withdrawn version named again stays withdrawn",
			steps: []step{
				{at: 0, version: "1.0.0", eff: -day, title: "v1"},
				{at: hour, version: "2.0.0", eff: 30 * day, title: "v2"},
				{at: 2 * hour, version: "1.0.0", eff: -day, title: "v1"},
				{at: 3 * hour, version: "2.0.0", eff: 30 * day, title: "v2"},
			},
			at:        60 * day,
			current:   "1.0.0",
			withdrawn: []string{"2.0.0"},
			results:   []bool{true, true, true, true},
		},
		{
			name: "a withdrawn version named again with a past date does not take effect",
			steps: []step{
				{at: 0, version: "1.0.0", eff: -day, title: "v1"},
				{at: hour, version: "2.0.0", eff: 30 * day, title: "v2"},
				{at: 2 * hour, version: "1.0.0", eff: -day, title: "v1"},
				{at: 3 * hour, version: "2.0.0", eff: -day, title: "v2"},
			},
			at:        4 * hour,
			current:   "1.0.0",
			withdrawn: []string{"2.0.0"},
			results:   []bool{true, true, true, true},
		},
		{
			name: "naming a withdrawn version also withdraws the other upcoming version",
			steps: []step{
				{at: 0, version: "2.0.0", eff: 30 * day, title: "v2"},
				{at: hour, version: "3.0.0", eff: 30 * day, title: "v3"},
				{at: 2 * hour, version: "2.0.0", eff: 30 * day, title: "v2"},
			},
			at:        2 * hour,
			withdrawn: []string{"2.0.0", "3.0.0"},
			results:   []bool{true, true, true},
		},
		{
			name: "an upcoming version that took effect is not withdrawn by the next one",
			steps: []step{
				{at: 0, version: "1.0.0", eff: -day, title: "v1"},
				{at: hour, version: "2.0.0", eff: day, title: "v2"},
				{at: 2 * day, version: "3.0.0", eff: 30 * day, title: "v3"},
			},
			at:       2 * day,
			current:  "2.0.0",
			upcoming: "3.0.0", upcomingTitle: "v3",
			results: []bool{true, true, true},
		},
		{
			name: "an old version named with a future date stays in effect when the next version withdraws it",
			steps: []step{
				{at: 0, version: "1.0.0", eff: -day, title: "v1"},
				{at: hour, version: "1.0.0", eff: 30 * day, title: "v1"},
				{at: 2 * hour, version: "1.1.0", eff: 30 * day, title: "v1.1"},
			},
			at:       3 * hour,
			current:  "1.0.0",
			upcoming: "1.1.0", upcomingTitle: "v1.1",
			results: []bool{true, true, true},
		},
		{
			name: "a correction that moves the date into the past takes effect",
			steps: []step{
				{at: 0, version: "1.0.0", eff: -day, title: "v1"},
				{at: hour, version: "2.0.0", eff: 30 * day, title: "v2"},
				{at: 2 * hour, version: "2.0.0", eff: hour, title: "v2"},
			},
			at:      3 * hour,
			current: "2.0.0",
			results: []bool{true, true, true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeRepo()
			res := run(t, f, tt.steps)
			for i, want := range tt.results {
				assert.Equal(t, want, len(res[i].Applied) == 1, "step %d applied", i)
			}
			c := f.catalog()
			now := t0.Add(tt.at)
			assert.Equal(t, tt.current, versionOf(c.Current("tos", now)), "current")
			up := c.Upcoming("tos", now)
			assert.Equal(t, tt.upcoming, versionOf(up), "upcoming")
			if up != nil {
				assert.Equal(t, tt.upcomingTitle, up.Title["en"])
			}
			for _, v := range tt.withdrawn {
				assert.True(t, c.Withdrawn(Ref{Type: "tos", Version: MustParseVersion(v)}), "%s withdrawn", v)
			}
			for _, v := range tt.absent {
				_, ok := f.versions[Ref{Type: "tos", Version: MustParseVersion(v)}]
				assert.False(t, ok, "%s must not be stored", v)
			}
		})
	}
}

func TestApply_VersionInEffectKeepsItsContent(t *testing.T) {
	f := newFakeRepo()
	run(t, f, []step{
		{at: 0, version: "1.0.0", eff: -day, title: "v1"},
		{at: hour, version: "1.0.0", eff: -2 * day, title: "rewritten"},
	})
	got := f.catalog().Current("tos", t0.Add(hour))
	require.NotNil(t, got)
	assert.Equal(t, "v1", got.Title["en"])
	assert.True(t, got.EffectiveAt.Equal(t0.Add(-day)))
	assert.Equal(t, "v1", f.versions[got.Ref].Title["en"])
}

func TestApply_ReplayKeepsTheHead(t *testing.T) {
	f := newFakeRepo()
	run(t, f, []step{{at: 0, version: "2.0.0", eff: 30 * day, title: "v2"}})
	before := f.heads["tos"]
	res := run(t, f, []step{{at: 0, version: "2.0.0", eff: 30 * day, title: "v2"}})
	assert.Equal(t, []Type{"tos"}, res[0].Ignored)
	assert.Equal(t, before, f.heads["tos"])
}

func TestApply_FailedArchiveWriteDecidesAndIsRepairedByTheNextApply(t *testing.T) {
	f := newFakeRepo()
	run(t, f, []step{{at: 0, version: "1.0.0", eff: -day, title: "v1"}})
	f.failArchive = true
	m := Manifest{Timestamp: t0.Add(hour), Entries: []Entry{entry("2.0.0", day, "v2")}}
	res, err := Apply(context.Background(), f, m, func() time.Time { return t0.Add(hour) })
	assert.Error(t, err)
	assert.Equal(t, []Type{"tos"}, res.Applied, "the head decided before the archive failed")

	c := f.catalog()
	assert.Equal(t, "2.0.0", versionOf(c.Upcoming("tos", t0.Add(hour))))
	assert.Equal(t, "2.0.0", versionOf(c.Current("tos", t0.Add(2*day))))
	_, archived := f.versions[Ref{Type: "tos", Version: MustParseVersion("2.0.0")}]
	assert.False(t, archived)

	run(t, f, []step{{at: 2 * day, version: "2.0.0", eff: day, title: "v2"}})
	assert.Equal(t, "v2", f.versions[Ref{Type: "tos", Version: MustParseVersion("2.0.0")}].Title["en"], "the next apply archives it")
}

func TestApply_InEffectPatchThenNewerRevert(t *testing.T) {
	patch := step{at: hour, clock: 2 * hour, version: "1.0.1", eff: hour, title: "v1.0.1"}
	revert := step{at: 2 * hour, version: "1.0.0", eff: -day, title: "v1"}
	base := step{at: 0, clock: 2 * hour, version: "1.0.0", eff: -day, title: "v1"}

	older := newFakeRepo()
	run(t, older, []step{base, patch, revert})
	assert.Equal(t, "1.0.1", versionOf(older.catalog().Current("tos", t0.Add(2*hour))), "a version in effect never changes")

	newer := newFakeRepo()
	run(t, newer, []step{base, revert, patch})
	assert.Equal(t, "1.0.0", versionOf(newer.catalog().Current("tos", t0.Add(2*hour))), "the older patch is ignored")
}

func TestPlanApply_PromotesTheUpcomingVersionBeforeApplying(t *testing.T) {
	v1 := entry("1.0.0", -day, "v1").record()
	v2 := entry("2.0.0", day, "v2").record()
	head := TypeHead{Type: "tos", Revision: 2, AppliedAt: t0, Current: &v1, Upcoming: &v2}

	next, ok := PlanApply(head, entry("1.0.0", -day, "v1"), t0.Add(2*day), t0.Add(2*day))
	require.True(t, ok)
	assert.Equal(t, "2.0.0", versionOf(next.Current), "2.0.0 took effect before the revert arrived")
	assert.Nil(t, next.Upcoming)
	assert.Empty(t, next.Withdrawn)
}

func TestPlanApply_EqualTimestamp(t *testing.T) {
	a, b := entry("2.0.0", 30*day, "draft"), entry("2.0.0", 40*day, "final")
	if a.applied().beats(b.applied()) {
		a, b = b, a
	}
	first, ok := PlanApply(TypeHead{Type: "tos"}, a, t0, t0)
	require.True(t, ok)

	_, ok = PlanApply(first, a, t0, t0)
	assert.False(t, ok, "an identical entry is a replay")

	next, ok := PlanApply(first, b, t0, t0)
	require.True(t, ok, "the larger digest wins at the same version")
	assert.Equal(t, b.Title, next.Upcoming.Title)

	_, ok = PlanApply(next, a, t0, t0)
	assert.False(t, ok, "the smaller digest loses after the larger")

	_, ok = PlanApply(next, entry("1.9.0", 30*day, "lower"), t0, t0)
	assert.False(t, ok, "a lower version loses")
	_, ok = PlanApply(next, entry("2.0.0", 30*day, "v2"), t0.Add(-time.Millisecond), t0)
	assert.False(t, ok, "an older payload loses")
}
