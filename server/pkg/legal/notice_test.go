package legal_test

import (
	"testing"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/legal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNotice(t *testing.T) {
	now := base.Add(day)
	in := func(typ, v string, required bool) legal.Entry { return entry(typ, v, base, required) }
	ahead := func(typ, v string, required bool) legal.Entry { return entry(typ, v, base.Add(10*day), required) }
	admin := id.NewAdminUserID()

	tests := []struct {
		name     string
		entries  []delivery
		wantErr  error
		wantBump legal.Bump
		wantRef  legal.Ref
	}{
		{name: "unknown type", wantErr: legal.ErrNoUpcoming},
		{
			name:    "no upcoming version",
			entries: []delivery{{at: base, entries: []legal.Entry{in("tos", "1.0.0", true)}}},
			wantErr: legal.ErrNoUpcoming,
		},
		{
			name: "upcoming version not required",
			entries: []delivery{
				{at: base, entries: []legal.Entry{in("tos", "1.0.0", true)}},
				{at: base.Add(1), entries: []legal.Entry{ahead("tos", "2.0.0", false)}},
			},
			wantErr: legal.ErrNotRequired,
		},
		{
			name: "patch",
			entries: []delivery{
				{at: base, entries: []legal.Entry{in("tos", "1.2.0", true)}},
				{at: base.Add(1), entries: []legal.Entry{ahead("tos", "1.2.1", true)}},
			},
			wantErr: legal.ErrPatchNotice,
		},
		{
			name: "minor",
			entries: []delivery{
				{at: base, entries: []legal.Entry{in("tos", "1.2.3", true)}},
				{at: base.Add(1), entries: []legal.Entry{ahead("tos", "1.3.0", true)}},
			},
			wantBump: legal.BumpMinor,
			wantRef:  ref("tos", "1.3.0"),
		},
		{
			name: "major",
			entries: []delivery{
				{at: base, entries: []legal.Entry{in("tos", "1.2.3", true)}},
				{at: base.Add(1), entries: []legal.Entry{ahead("tos", "2.0.0", true)}},
			},
			wantBump: legal.BumpMajor,
			wantRef:  ref("tos", "2.0.0"),
		},
		{
			name:     "no current version counts as major",
			entries:  []delivery{{at: base, entries: []legal.Entry{ahead("tos", "1.0.0", true)}}},
			wantBump: legal.BumpMajor,
			wantRef:  ref("tos", "1.0.0"),
		},
		{
			name: "the current version being optional does not matter",
			entries: []delivery{
				{at: base, entries: []legal.Entry{in("tos", "1.0.0", false)}},
				{at: base.Add(1), entries: []legal.Entry{ahead("tos", "1.1.0", true)}},
			},
			wantBump: legal.BumpMinor,
			wantRef:  ref("tos", "1.1.0"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := legal.NewNotice(catalog(t, tt.entries...), "tos", legal.AudienceAll, admin, now)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantBump, n.Bump)
			assert.Equal(t, tt.wantRef, n.Ref)
			assert.Equal(t, legal.AudienceAll, n.Audience)
			assert.Equal(t, admin, n.RequestedBy)
			assert.Equal(t, now, n.CreatedAt)
			assert.False(t, n.ID.IsNil())
			assert.Equal(t, legal.Fanout{}, n.Fanout)
		})
	}
}

func TestParseAudience(t *testing.T) {
	for _, s := range []string{"all", "owners"} {
		a, err := legal.ParseAudience(s)
		require.NoError(t, err)
		assert.Equal(t, legal.Audience(s), a)
	}
	for _, s := range []string{"", "All", "members"} {
		_, err := legal.ParseAudience(s)
		assert.ErrorIs(t, err, legal.ErrInvalidAudience, s)
	}
}

func TestHoldsAtLeast(t *testing.T) {
	c := catalog(t,
		delivery{at: base, entries: []legal.Entry{entry("tos", "1.0.0", base, true)}},
		delivery{at: base.Add(1), entries: []legal.Entry{entry("tos", "2.0.0", base.Add(10*day), true)}},
	)
	up := ref("tos", "2.0.0")
	assert.False(t, legal.HoldsAtLeast(c, nil, up))
	assert.False(t, legal.HoldsAtLeast(c, accepted(ref("tos", "1.0.0")), up))
	assert.True(t, legal.HoldsAtLeast(c, accepted(ref("tos", "1.0.0"), ref("tos", "2.0.0")), up))
	assert.False(t, legal.HoldsAtLeast(c, accepted(ref("privacy", "2.0.0")), up), "another type does not count")
}
