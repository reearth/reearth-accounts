package conformance

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/legal"
	"github.com/reearth/reearth-accounts/server/pkg/outbox"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runPolicyNotice(t *testing.T, nc Factory) {
	t.Run("PolicyNotice_CreateIdempotent", func(t *testing.T) { testPolicyNoticeCreateIdempotent(t, nc) })
	t.Run("PolicyNotice_FanoutLease", func(t *testing.T) { testPolicyNoticeFanoutLease(t, nc) })
	t.Run("PolicyNotice_ClaimExclusive", func(t *testing.T) { testPolicyNoticeClaimExclusive(t, nc) })
	t.Run("PolicyNotice_OutboxCountByNotice", func(t *testing.T) { testOutboxCountByNotice(t, nc) })
	t.Run("PolicyNotice_AcceptancesByUsers", func(t *testing.T) { testAcceptancesByUsers(t, nc) })
	t.Run("Audience_ActiveUsersWithEmail", func(t *testing.T) { testAudienceActiveUsers(t, nc) })
	t.Run("Audience_OrgOwners", func(t *testing.T) { testAudienceOrgOwners(t, nc) })
}

func notice(ref legal.Ref, a legal.Audience, at time.Time) legal.Notice {
	return legal.Notice{
		Audience: a, Bump: legal.BumpMajor, CreatedAt: at, ID: id.NewPolicyNoticeID(), Ref: ref, RequestedBy: id.NewAdminUserID(),
	}
}

func tos(v string) legal.Ref { return legal.Ref{Type: "tos", Version: legal.MustParseVersion(v)} }

func testPolicyNoticeCreateIdempotent(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	now := timeFixed()

	first := notice(tos("2.0.0"), legal.AudienceAll, now)
	got, created, err := c.PolicyNotice.Create(ctx, first)
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, first, got)

	again := notice(tos("2.0.0"), legal.AudienceAll, now.Add(time.Hour))
	got, created, err = c.PolicyNotice.Create(ctx, again)
	require.NoError(t, err)
	assert.False(t, created, "a second notice of the same version and audience is not stored")
	assert.Equal(t, first, got, "the stored notice comes back")

	owners := notice(tos("2.0.0"), legal.AudienceOwners, now.Add(time.Minute))
	_, created, err = c.PolicyNotice.Create(ctx, owners)
	require.NoError(t, err)
	assert.True(t, created, "another audience is another notice")
	_, _, err = c.PolicyNotice.Create(ctx, notice(tos("3.0.0"), legal.AudienceAll, now))
	require.NoError(t, err)

	list, err := c.PolicyNotice.FindByRef(ctx, tos("2.0.0"))
	require.NoError(t, err)
	assert.Equal(t, []legal.Notice{first, owners}, list)
	list, err = c.PolicyNotice.FindByRef(ctx, tos("1.0.0"))
	require.NoError(t, err)
	assert.Empty(t, list)
}

func testPolicyNoticeFanoutLease(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	now := timeFixed()
	older := notice(tos("2.0.0"), legal.AudienceAll, now)
	newer := notice(tos("2.0.0"), legal.AudienceOwners, now.Add(time.Second))
	for _, n := range []legal.Notice{newer, older} {
		_, _, err := c.PolicyNotice.Create(ctx, n)
		require.NoError(t, err)
	}

	a := legal.Lease{Token: "a", Until: now.Add(time.Minute)}
	got, err := c.PolicyNotice.ClaimFanout(ctx, now, a)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, older.ID, got.ID, "the oldest notice first")
	assert.Equal(t, &a, got.Fanout.Lease)

	b := legal.Lease{Token: "b", Until: now.Add(time.Minute)}
	got, err = c.PolicyNotice.ClaimFanout(ctx, now, b)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, newer.ID, got.ID, "a leased notice is skipped")
	got, err = c.PolicyNotice.ClaimFanout(ctx, now, legal.Lease{Token: "c", Until: now.Add(time.Minute)})
	require.NoError(t, err)
	assert.Nil(t, got, "both notices are leased")

	ok, err := c.PolicyNotice.AdvanceFanout(ctx, older.ID, legal.Lease{Token: "stale", Until: now.Add(2 * time.Minute)}, "u1", false)
	require.NoError(t, err)
	assert.False(t, ok, "a stale token writes nothing")
	renewed := legal.Lease{Token: "a", Until: now.Add(3 * time.Minute)}
	ok, err = c.PolicyNotice.AdvanceFanout(ctx, older.ID, renewed, "u1", false)
	require.NoError(t, err)
	assert.True(t, ok)

	got, err = c.PolicyNotice.ClaimFanout(ctx, now.Add(2*time.Minute), legal.Lease{Token: "d", Until: now.Add(4 * time.Minute)})
	require.NoError(t, err)
	require.NotNil(t, got, "the newer notice's lease expired at its until")
	assert.Equal(t, newer.ID, got.ID)
	ok, err = c.PolicyNotice.AdvanceFanout(ctx, newer.ID, b, "w", false)
	require.NoError(t, err)
	assert.False(t, ok, "the first holder lost the notice when its lease expired")

	got, err = c.PolicyNotice.ClaimFanout(ctx, now.Add(3*time.Minute), legal.Lease{Token: "e", Until: now.Add(5 * time.Minute)})
	require.NoError(t, err)
	require.NotNil(t, got, "the renewed lease ends at its new until")
	assert.Equal(t, older.ID, got.ID)
	assert.Equal(t, "u1", got.Fanout.Cursor, "the cursor survives a lost lease")

	ok, err = c.PolicyNotice.AdvanceFanout(ctx, older.ID, legal.Lease{Token: "e", Until: now.Add(5 * time.Minute)}, "u9", true)
	require.NoError(t, err)
	assert.True(t, ok)
	require.NoError(t, c.PolicyNotice.ReleaseFanout(ctx, newer.ID, "d"))

	list, err := c.PolicyNotice.FindByRef(ctx, tos("2.0.0"))
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, legal.Fanout{Cursor: "u9", Done: true}, list[0].Fanout, "a done notice drops its lease")
	assert.Equal(t, legal.Fanout{}, list[1].Fanout, "a release drops the lease")

	got, err = c.PolicyNotice.ClaimFanout(ctx, now.Add(3*time.Minute), legal.Lease{Token: "f", Until: now.Add(time.Hour)})
	require.NoError(t, err)
	require.NotNil(t, got, "a released notice can be claimed at once")
	assert.Equal(t, newer.ID, got.ID)
	got, err = c.PolicyNotice.ClaimFanout(ctx, now.Add(2*time.Hour), legal.Lease{Token: "g", Until: now.Add(3 * time.Hour)})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, newer.ID, got.ID, "a done notice is never claimed")
}

func testPolicyNoticeClaimExclusive(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	now := timeFixed()
	const notices = 6
	for i := range notices {
		_, _, err := c.PolicyNotice.Create(ctx, notice(tos(fmt.Sprintf("%d.0.0", i+2)), legal.AudienceAll, now.Add(time.Duration(i)*time.Second)))
		require.NoError(t, err)
	}

	var mu sync.Mutex
	claimed := map[id.PolicyNoticeID]int{}
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				n, err := c.PolicyNotice.ClaimFanout(ctx, now, legal.Lease{Token: fmt.Sprintf("g%d", g), Until: now.Add(time.Hour)})
				if !assert.NoError(t, err) || n == nil {
					return
				}
				mu.Lock()
				claimed[n.ID]++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	assert.Len(t, claimed, notices)
	for n, times := range claimed {
		assert.Equal(t, 1, times, "notice %s claimed once", n)
	}
}

func testOutboxCountByNotice(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	now := timeFixed()
	n, other := id.NewPolicyNoticeID(), id.NewPolicyNoticeID()
	subject := func(nid id.PolicyNoticeID) outbox.Subject {
		return outbox.PolicyNoticeSubject{Bump: legal.BumpMajor, Notice: nid, Ref: tos("2.0.0")}.Subject()
	}
	var rs []outbox.Record
	for i, nid := range []id.PolicyNoticeID{n, n, n, other} {
		r, err := outbox.New(outbox.Key(fmt.Sprintf("k%d", i)), testKind, subject(nid), outbox.ToUser(id.NewUserID()), now)
		require.NoError(t, err)
		rs = append(rs, r)
	}
	require.NoError(t, c.Outbox.Enqueue(ctx, rs))

	cl := claim("t", now.Add(time.Minute))
	rec, err := c.Outbox.ClaimDue(ctx, now, cl)
	require.NoError(t, err)
	require.NotNil(t, rec)
	ok, err := c.Outbox.Complete(ctx, rec.Key, "t", outbox.Transition{To: outbox.StateSent}, now)
	require.NoError(t, err)
	require.True(t, ok)

	counts, err := c.Outbox.CountByNotice(ctx, n)
	require.NoError(t, err)
	want := outbox.EmptyCounts()
	if rec.Subject[outbox.SubjectNotice] == n.String() {
		want[outbox.StatePending], want[outbox.StateSent] = 2, 1
	} else {
		want[outbox.StatePending] = 3
	}
	assert.Equal(t, want, counts)

	counts, err = c.Outbox.CountByNotice(ctx, id.NewPolicyNoticeID())
	require.NoError(t, err)
	assert.Equal(t, outbox.EmptyCounts(), counts, "every state is present at zero")
}

func testAcceptancesByUsers(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	u1, u2, u3 := id.NewUserID(), id.NewUserID(), id.NewUserID()
	require.NoError(t, c.PolicyAcceptance.InsertIfAbsent(ctx, []legal.Acceptance{
		acceptance(u1, "tos", "1.0.0", "a"),
		acceptance(u1, "tos", "2.0.0", "a"),
		acceptance(u1, "privacy", "1.0.0", "a"),
		acceptance(u2, "tos", "1.0.0", "a"),
		acceptance(u3, "tos", "2.0.0", "a"),
	}))

	got, err := c.PolicyAcceptance.FindByUsers(ctx, []id.UserID{u1, u2, id.NewUserID()}, "tos")
	require.NoError(t, err)
	keys := make([]string, 0, len(got))
	for _, a := range got {
		keys = append(keys, a.UserID.String()+":"+a.Version.String())
	}
	slices.Sort(keys)
	want := []string{u1.String() + ":1.0.0", u1.String() + ":2.0.0", u2.String() + ":1.0.0"}
	slices.Sort(want)
	assert.Equal(t, want, keys)

	got, err = c.PolicyAcceptance.FindByUsers(ctx, nil, "tos")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func sortedIDs[T interface{ String() string }](ids []T) []T {
	out := slices.Clone(ids)
	slices.SortFunc(out, func(a, b T) int { return strings.Compare(a.String(), b.String()) })
	return out
}

func testAudienceActiveUsers(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	var active user.IDList
	for i := range 5 {
		u := newUser(t, fmt.Sprintf("active%d", i), fmt.Sprintf("active%d@example.com", i))
		require.NoError(t, c.User.Save(ctx, u))
		active = append(active, u.ID())
	}
	deleted := newUser(t, "deleted", "deleted@example.com")
	deleted.Deactivate()
	require.NoError(t, c.User.Save(ctx, deleted))
	active = sortedIDs(active)

	var paged user.IDList
	var after *user.ID
	for pages := 0; ; pages++ {
		require.Less(t, pages, 10)
		ids, err := c.User.FindActiveIDsWithEmailAfter(ctx, after, 2)
		require.NoError(t, err)
		if len(ids) == 0 {
			break
		}
		assert.LessOrEqual(t, len(ids), 2)
		paged = append(paged, ids...)
		after = &ids[len(ids)-1]
	}
	assert.Equal(t, active, paged, "every active user with an email, once, in ID order")

	ids, err := c.User.FindActiveIDsWithEmailAfter(ctx, &active[2], 10)
	require.NoError(t, err)
	assert.Equal(t, active[3:], ids)
}

func testAudienceOrgOwners(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	member := func(r role.RoleType, disabled bool) workspace.Member {
		return workspace.Member{Disabled: disabled, Role: r}
	}
	org := func(members map[id.UserID]workspace.Member, mod func(*workspace.Builder)) *workspace.Workspace {
		b := workspace.New().NewID().Name("org").Members(members)
		if mod != nil {
			mod(b)
		}
		ws, err := b.Build()
		require.NoError(t, err)
		require.NoError(t, c.Workspace.Save(ctx, ws))
		return ws
	}

	o1, o2, o3, writer, disabled, personalOwner, deletedOwner := id.NewUserID(), id.NewUserID(), id.NewUserID(),
		id.NewUserID(), id.NewUserID(), id.NewUserID(), id.NewUserID()
	w1 := org(map[id.UserID]workspace.Member{o1: member(role.RoleOwner, false), writer: member(role.RoleWriter, false)}, nil)
	w2 := org(map[id.UserID]workspace.Member{o2: member(role.RoleOwner, false), o1: member(role.RoleOwner, false), disabled: member(role.RoleOwner, true)}, nil)
	w3 := org(map[id.UserID]workspace.Member{o3: member(role.RoleOwner, false)}, nil)
	org(map[id.UserID]workspace.Member{personalOwner: member(role.RoleOwner, false)}, func(b *workspace.Builder) { b.Personal(true) })
	deletedAt := timeFixed()
	org(map[id.UserID]workspace.Member{deletedOwner: member(role.RoleOwner, false)}, func(b *workspace.Builder) { b.DeletedAt(&deletedAt) })

	orgs := sortedIDs([]workspace.ID{w1.ID(), w2.ID(), w3.ID()})
	ownersOf := map[workspace.ID][]user.ID{w1.ID(): {o1}, w2.ID(): {o1, o2}, w3.ID(): {o3}}

	var after *workspace.ID
	var seen []workspace.ID
	got := map[user.ID]int{}
	for pages := 0; ; pages++ {
		require.Less(t, pages, 10)
		owners, last, err := c.Workspace.FindOrgOwnersAfter(ctx, after, 2)
		require.NoError(t, err)
		if last == nil {
			assert.Empty(t, owners)
			break
		}
		seen = append(seen, *last)
		for _, o := range owners {
			got[o]++
		}
		after = last
	}
	assert.Equal(t, []workspace.ID{orgs[1], orgs[2]}, seen, "pages of two end at the second and third workspace")
	want := map[user.ID]int{}
	for _, w := range orgs {
		for _, o := range ownersOf[w] {
			want[o]++
		}
	}
	assert.Equal(t, want, got, "enabled owners of non-personal, non-deleted workspaces")

	owners, last, err := c.Workspace.FindOrgOwnersAfter(ctx, &orgs[1], 5)
	require.NoError(t, err)
	require.NotNil(t, last)
	assert.Equal(t, orgs[2], *last)
	assert.ElementsMatch(t, ownersOf[orgs[2]], owners)
}
