package legal

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var t0 = time.Date(2026, 11, 20, 3, 0, 0, 0, time.UTC)

// fakeRepo is a sequential VersionRepo that honors the repo contract.
type fakeRepo struct {
	heads    map[Type]TypeHead
	versions map[Ref]VersionRecord
	dropNext bool
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{heads: map[Type]TypeHead{}, versions: map[Ref]VersionRecord{}}
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

func (f *fakeRepo) UpsertVersion(_ context.Context, r VersionRecord, now time.Time) error {
	if f.dropNext {
		f.dropNext = false
		return nil
	}
	if cur, ok := f.versions[r.Ref]; ok && !cur.CanOverwrite(r.Stamp, now) {
		return nil
	}
	f.versions[r.Ref] = r
	return nil
}

func (f *fakeRepo) catalog() Catalog {
	hs, _ := f.FindHeads(context.Background())
	rs, _ := f.FindVersions(context.Background())
	return NewCatalog(hs, rs)
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
				{at: 0, version: "1.0.0", eff: -day, title: "v1"},
				{at: 0, version: "2.0.0", eff: 30 * day, title: "v2"},
			},
			current: "1.0.0",
			absent:  []string{"2.0.0"},
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

func TestApply_FrozenRecordKeepsItsContent(t *testing.T) {
	f := newFakeRepo()
	run(t, f, []step{
		{at: 0, version: "1.0.0", eff: -day, title: "v1"},
		{at: hour, version: "1.0.0", eff: -2 * day, title: "rewritten"},
	})
	got := f.versions[Ref{Type: "tos", Version: MustParseVersion("1.0.0")}]
	assert.Equal(t, "v1", got.Title["en"])
	assert.True(t, got.EffectiveAt.Equal(t0.Add(-day)))
}

func TestApply_ReplayKeepsTheHead(t *testing.T) {
	f := newFakeRepo()
	run(t, f, []step{{at: 0, version: "2.0.0", eff: 30 * day, title: "v2"}})
	before := f.heads["tos"]
	res := run(t, f, []step{{at: 0, version: "2.0.0", eff: 30 * day, title: "v2"}})
	assert.Equal(t, []Type{"tos"}, res[0].Ignored)
	assert.Equal(t, before, f.heads["tos"])
}

func TestApply_ReplayRepairsARecordLostAfterTheHeadWon(t *testing.T) {
	f := newFakeRepo()
	run(t, f, []step{{at: 0, version: "1.0.0", eff: -day, title: "v1"}})
	f.dropNext = true
	run(t, f, []step{{at: hour, version: "2.0.0", eff: 30 * day, title: "v2"}})

	now := t0.Add(hour)
	c := f.catalog()
	assert.Nil(t, c.Upcoming("tos", now), "a head pointer without its record is ignored")
	assert.Equal(t, "1.0.0", versionOf(c.Current("tos", now)))

	run(t, f, []step{{at: hour, version: "2.0.0", eff: 30 * day, title: "v2"}})
	assert.Equal(t, "2.0.0", versionOf(f.catalog().Upcoming("tos", now)))
}

func TestPlanApply_ReplayWritesOnlyWhatTheHeadNames(t *testing.T) {
	head := TypeHead{
		Type: "tos", Revision: 3, AppliedAt: t0,
		Upcoming: &Scheduled{Version: MustParseVersion("2.0.0"), EffectiveAt: t0.Add(30 * day)},
	}
	assert.Equal(t, Decision{Write: true}, PlanApply(head, entry("2.0.0", 30*day, "v2"), t0, t0))
	assert.Equal(t, Decision{}, PlanApply(head, entry("2.0.0", 31*day, "v2"), t0, t0))
	assert.Equal(t, Decision{}, PlanApply(head, entry("3.0.0", 30*day, "v3"), t0, t0))
	assert.Equal(t, Decision{}, PlanApply(head, entry("1.0.0", -day, "v1"), t0, t0))
	assert.Equal(t, Decision{}, PlanApply(head, entry("2.0.0", 30*day, "v2"), t0.Add(-time.Second), t0))
}
