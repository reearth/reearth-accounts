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

type fakeRepo struct {
	failArchive bool
	heads       map[Type]TypeHead
	revisions   map[Ref]int64
	versions    map[Ref]VersionRecord
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{heads: map[Type]TypeHead{}, revisions: map[Ref]int64{}, versions: map[Ref]VersionRecord{}}
}

func (f *fakeRepo) ArchiveVersions(_ context.Context, h TypeHead) error {
	if f.failArchive {
		f.failArchive = false
		return errors.New("archive unavailable")
	}
	for _, v := range h.Versions {
		if rev, ok := f.revisions[v.Ref]; ok && rev > h.Revision {
			continue
		}
		f.versions[v.Ref] = v.VersionRecord
		f.revisions[v.Ref] = h.Revision
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
	effectiveAt time.Duration
	now         time.Duration
	payloadAt   time.Duration
	title       string
	version     string
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
		m := Manifest{Timestamp: t0.Add(s.payloadAt), Entries: []Entry{entry(s.version, s.effectiveAt, s.title)}}
		res, err := Apply(context.Background(), f, m, func() time.Time { return t0.Add(s.now) })
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
		applied       []bool
		current       string
		upcoming      string
		upcomingTitle string
		withdrawn     []string
		readAt        time.Duration
	}{
		{
			name:     "first version already in effect becomes current",
			steps:    []step{{payloadAt: 0, now: 0, version: "1.0.0", effectiveAt: -day, title: "v1"}},
			current:  "1.0.0",
			applied:  []bool{true},
			upcoming: "",
		},
		{
			name:     "new version ahead of its date is upcoming and nothing is current yet",
			steps:    []step{{payloadAt: 0, now: 0, version: "2.0.0", effectiveAt: 30 * day, title: "v2"}},
			upcoming: "2.0.0", upcomingTitle: "v2",
			applied: []bool{true},
		},
		{
			name: "replay of the same payload changes nothing",
			steps: []step{
				{payloadAt: 0, now: 0, version: "2.0.0", effectiveAt: 30 * day, title: "v2"},
				{payloadAt: 0, now: hour, version: "2.0.0", effectiveAt: 30 * day, title: "v2"},
			},
			readAt:   hour,
			upcoming: "2.0.0", upcomingTitle: "v2",
			applied: []bool{true, false},
		},
		{
			name: "a same-timestamp loser already in effect is kept in effect",
			steps: []step{
				{payloadAt: 0, now: 0, version: "2.0.0", effectiveAt: 30 * day, title: "v2"},
				{payloadAt: 0, now: 0, version: "1.0.0", effectiveAt: -day, title: "v1"},
			},
			current:  "1.0.0",
			upcoming: "2.0.0", upcomingTitle: "v2",
			applied: []bool{true, false},
		},
		{
			name: "an older payload's version not yet in effect is recorded withdrawn",
			steps: []step{
				{payloadAt: hour, now: hour, version: "1.0.0", effectiveAt: -day, title: "v1"},
				{payloadAt: 0, now: 2 * hour, version: "2.0.0", effectiveAt: 30 * day, title: "v2"},
			},
			readAt:    2 * hour,
			current:   "1.0.0",
			withdrawn: []string{"2.0.0"},
			applied:   []bool{true, false},
		},
		{
			name: "reverting withdraws the upcoming version and it never takes effect",
			steps: []step{
				{payloadAt: 0, now: 0, version: "1.0.0", effectiveAt: -day, title: "v1"},
				{payloadAt: hour, now: hour, version: "2.0.0", effectiveAt: 30 * day, title: "v2"},
				{payloadAt: 2 * hour, now: 2 * hour, version: "1.0.0", effectiveAt: -day, title: "v1"},
			},
			readAt:    60 * day,
			current:   "1.0.0",
			withdrawn: []string{"2.0.0"},
			applied:   []bool{true, true, true},
		},
		{
			name: "a later payload corrects a version that has not taken effect",
			steps: []step{
				{payloadAt: 0, now: 0, version: "2.0.0", effectiveAt: 30 * day, title: "draft"},
				{payloadAt: hour, now: hour, version: "2.0.0", effectiveAt: 40 * day, title: "final"},
			},
			readAt:   35 * day,
			upcoming: "2.0.0", upcomingTitle: "final",
			applied: []bool{true, true},
		},
		{
			name: "a version in effect never changes",
			steps: []step{
				{payloadAt: 0, now: 0, version: "1.0.0", effectiveAt: -day, title: "v1"},
				{payloadAt: hour, now: hour, version: "1.0.0", effectiveAt: 30 * day, title: "rewritten"},
			},
			readAt:  2 * hour,
			current: "1.0.0",
			applied: []bool{true, true},
		},
		{
			name: "a withdrawn version named again stays withdrawn",
			steps: []step{
				{payloadAt: 0, now: 0, version: "1.0.0", effectiveAt: -day, title: "v1"},
				{payloadAt: hour, now: hour, version: "2.0.0", effectiveAt: 30 * day, title: "v2"},
				{payloadAt: 2 * hour, now: 2 * hour, version: "1.0.0", effectiveAt: -day, title: "v1"},
				{payloadAt: 3 * hour, now: 3 * hour, version: "2.0.0", effectiveAt: 30 * day, title: "v2"},
			},
			readAt:    60 * day,
			current:   "1.0.0",
			withdrawn: []string{"2.0.0"},
			applied:   []bool{true, true, true, true},
		},
		{
			name: "a withdrawn version named again with a past date does not take effect",
			steps: []step{
				{payloadAt: 0, now: 0, version: "1.0.0", effectiveAt: -day, title: "v1"},
				{payloadAt: hour, now: hour, version: "2.0.0", effectiveAt: 30 * day, title: "v2"},
				{payloadAt: 2 * hour, now: 2 * hour, version: "1.0.0", effectiveAt: -day, title: "v1"},
				{payloadAt: 3 * hour, now: 3 * hour, version: "2.0.0", effectiveAt: -day, title: "v2"},
			},
			readAt:    4 * hour,
			current:   "1.0.0",
			withdrawn: []string{"2.0.0"},
			applied:   []bool{true, true, true, true},
		},
		{
			name: "naming a withdrawn version also withdraws the other upcoming version",
			steps: []step{
				{payloadAt: 0, now: 0, version: "2.0.0", effectiveAt: 30 * day, title: "v2"},
				{payloadAt: hour, now: hour, version: "3.0.0", effectiveAt: 30 * day, title: "v3"},
				{payloadAt: 2 * hour, now: 2 * hour, version: "2.0.0", effectiveAt: 30 * day, title: "v2"},
			},
			readAt:    2 * hour,
			withdrawn: []string{"2.0.0", "3.0.0"},
			applied:   []bool{true, true, true},
		},
		{
			name: "an upcoming version that took effect is not withdrawn by the next one",
			steps: []step{
				{payloadAt: 0, now: 0, version: "1.0.0", effectiveAt: -day, title: "v1"},
				{payloadAt: hour, now: hour, version: "2.0.0", effectiveAt: day, title: "v2"},
				{payloadAt: 2 * day, now: 2 * day, version: "3.0.0", effectiveAt: 30 * day, title: "v3"},
			},
			readAt:   2 * day,
			current:  "2.0.0",
			upcoming: "3.0.0", upcomingTitle: "v3",
			applied: []bool{true, true, true},
		},
		{
			name: "an old version named with a future date stays in effect when the next version withdraws it",
			steps: []step{
				{payloadAt: 0, now: 0, version: "1.0.0", effectiveAt: -day, title: "v1"},
				{payloadAt: hour, now: hour, version: "1.0.0", effectiveAt: 30 * day, title: "v1"},
				{payloadAt: 2 * hour, now: 2 * hour, version: "1.1.0", effectiveAt: 30 * day, title: "v1.1"},
			},
			readAt:   3 * hour,
			current:  "1.0.0",
			upcoming: "1.1.0", upcomingTitle: "v1.1",
			applied: []bool{true, true, true},
		},
		{
			name: "a correction that moves the date into the past takes effect",
			steps: []step{
				{payloadAt: 0, now: 0, version: "1.0.0", effectiveAt: -day, title: "v1"},
				{payloadAt: hour, now: hour, version: "2.0.0", effectiveAt: 30 * day, title: "v2"},
				{payloadAt: 2 * hour, now: 2 * hour, version: "2.0.0", effectiveAt: hour, title: "v2"},
			},
			readAt:  3 * hour,
			current: "2.0.0",
			applied: []bool{true, true, true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeRepo()
			res := run(t, f, tt.steps)
			for i, want := range tt.applied {
				assert.Equal(t, want, len(res[i].Applied) == 1, "step %d applied", i)
			}
			c := f.catalog()
			now := t0.Add(tt.readAt)
			assert.Equal(t, tt.current, versionOf(c.Current("tos", now)), "current")
			up := c.Upcoming("tos", now)
			assert.Equal(t, tt.upcoming, versionOf(up), "upcoming")
			if up != nil {
				assert.Equal(t, tt.upcomingTitle, up.Title["en"])
			}
			for _, v := range tt.withdrawn {
				assert.True(t, c.Withdrawn(Ref{Type: "tos", Version: MustParseVersion(v)}), "%s withdrawn", v)
			}
		})
	}
}

func TestApply_VersionInEffectKeepsItsContent(t *testing.T) {
	f := newFakeRepo()
	run(t, f, []step{
		{payloadAt: 0, now: 0, version: "1.0.0", effectiveAt: -day, title: "v1"},
		{payloadAt: hour, now: hour, version: "1.0.0", effectiveAt: -2 * day, title: "rewritten"},
	})
	got := f.catalog().Current("tos", t0.Add(hour))
	require.NotNil(t, got)
	assert.Equal(t, "v1", got.Title["en"])
	assert.True(t, got.EffectiveAt.Equal(t0.Add(-day)))
	assert.Equal(t, "v1", f.versions[got.Ref].Title["en"])
}

func TestApply_ReplayKeepsTheHead(t *testing.T) {
	f := newFakeRepo()
	run(t, f, []step{{payloadAt: 0, now: 0, version: "2.0.0", effectiveAt: 30 * day, title: "v2"}})
	before := f.heads["tos"]
	res := run(t, f, []step{{payloadAt: 0, now: 0, version: "2.0.0", effectiveAt: 30 * day, title: "v2"}})
	assert.Equal(t, []Type{"tos"}, res[0].Ignored)
	assert.Equal(t, before, f.heads["tos"])
}

func TestApply_FailedArchiveWriteDecidesAndIsRepairedByTheNextApply(t *testing.T) {
	f := newFakeRepo()
	run(t, f, []step{{payloadAt: 0, now: 0, version: "1.0.0", effectiveAt: -day, title: "v1"}})
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

	run(t, f, []step{{payloadAt: 2 * day, now: 2 * day, version: "2.0.0", effectiveAt: day, title: "v2"}})
	assert.Equal(t, "v2", f.versions[Ref{Type: "tos", Version: MustParseVersion("2.0.0")}].Title["en"], "the next apply archives it")
}

func TestApply_InEffectPatchThenNewerRevert(t *testing.T) {
	patch := step{payloadAt: hour, now: 2 * hour, version: "1.0.1", effectiveAt: hour, title: "v1.0.1"}
	revert := step{payloadAt: 2 * hour, now: 2 * hour, version: "1.0.0", effectiveAt: -day, title: "v1"}
	base := step{payloadAt: 0, now: 2 * hour, version: "1.0.0", effectiveAt: -day, title: "v1"}

	for _, order := range [][]step{{base, patch, revert}, {base, revert, patch}} {
		f := newFakeRepo()
		run(t, f, order)
		assert.Equal(t, "1.0.1", versionOf(f.catalog().Current("tos", t0.Add(2*hour))), "a version in effect never changes")
	}
}

func TestTypeHead_At(t *testing.T) {
	withdrawnAt := t0
	head := TypeHead{Type: "tos", Versions: map[Version]HeadVersion{}}
	for _, v := range []HeadVersion{
		{VersionRecord: entry("1.0.0", -2*day, "v1").record()},
		{VersionRecord: entry("1.5.0", -3*day, "v1.5").record()},
		{VersionRecord: entry("2.0.0", day, "v2").record()},
		{VersionRecord: entry("3.0.0", -hour, "v3").record(), WithdrawnAt: &withdrawnAt},
	} {
		head.Versions[v.Version] = v
	}

	cur, up := head.At(t0)
	assert.Equal(t, "1.0.0", versionOf(cur), "the latest date that has passed, not the highest version")
	assert.Equal(t, "2.0.0", versionOf(up))

	cur, up = head.At(t0.Add(day))
	assert.Equal(t, "2.0.0", versionOf(cur), "an upcoming version is current at its date")
	assert.Nil(t, up)
}

func TestPlanApply_EqualTimestamp(t *testing.T) {
	a, b := entry("2.0.0", 30*day, "draft"), entry("2.0.0", 40*day, "final")
	if a.applied().beats(b.applied()) {
		a, b = b, a
	}
	first, outcome := PlanApply(TypeHead{Type: "tos"}, a, t0, t0)
	require.Equal(t, OutcomeWon, outcome)

	_, outcome = PlanApply(first, a, t0, t0)
	assert.Equal(t, OutcomeIgnored, outcome, "an identical entry is a replay")

	next, outcome := PlanApply(first, b, t0, t0)
	require.Equal(t, OutcomeWon, outcome, "the larger digest wins at the same version")
	_, up := next.At(t0)
	assert.Equal(t, b.Title, up.Title)

	_, outcome = PlanApply(next, a, t0, t0)
	assert.Equal(t, OutcomeIgnored, outcome, "the smaller digest loses after the larger")

	lower, outcome := PlanApply(next, entry("1.9.0", 30*day, "lower"), t0, t0)
	assert.Equal(t, OutcomeRecorded, outcome, "a lower version loses and is recorded")
	assert.True(t, lower.IsWithdrawn(MustParseVersion("1.9.0")))
	_, outcome = PlanApply(next, entry("2.0.0", 30*day, "v2"), t0.Add(-time.Millisecond), t0)
	assert.Equal(t, OutcomeIgnored, outcome, "an older payload naming a seen version changes nothing")
}
