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
	t.Run("PolicyNotice_ReopenWithdrawnOrCovered", func(t *testing.T) { testPolicyNoticeReopenWithdrawnOrCovered(t, nc) })
	t.Run("PolicyNotice_Withdraw", func(t *testing.T) { testPolicyNoticeWithdraw(t, nc) })
	t.Run("PolicyNotice_ClaimExclusive", func(t *testing.T) { testPolicyNoticeClaimExclusive(t, nc) })
	t.Run("PolicyNotice_OutboxCountByNotice", func(t *testing.T) { testOutboxCountByNotice(t, nc) })
	t.Run("PolicyNotice_OutboxRemoveFailedByNotice", func(t *testing.T) { testOutboxRemoveFailedByNotice(t, nc) })
	t.Run("PolicyNotice_AcceptancesByUsers", func(t *testing.T) { testAcceptancesByUsers(t, nc) })
	t.Run("Audience_ActiveUsersWithEmail", func(t *testing.T) { testAudienceActiveUsers(t, nc) })
	t.Run("Audience_OrgOwners", func(t *testing.T) { testAudienceOrgOwners(t, nc) })
	t.Run("Audience_OrgOwnersAmong", func(t *testing.T) { testAudienceOrgOwnersAmong(t, nc) })
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
	got, outcome, err := c.PolicyNotice.Create(ctx, first)
	require.NoError(t, err)
	assert.Equal(t, legal.NoticeCreated, outcome)
	assert.Equal(t, first, got)

	again := notice(tos("2.0.0"), legal.AudienceAll, now.Add(time.Hour))
	got, outcome, err = c.PolicyNotice.Create(ctx, again)
	require.NoError(t, err)
	assert.Equal(t, legal.NoticeExisting, outcome, "a second notice of the same version and audience is not stored")
	assert.Equal(t, first, got, "the stored notice comes back")

	owners := notice(tos("2.0.0"), legal.AudienceOwners, now.Add(time.Minute))
	_, outcome, err = c.PolicyNotice.Create(ctx, owners)
	require.NoError(t, err)
	assert.Equal(t, legal.NoticeCreated, outcome, "another audience is another notice")
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

	ok, err := c.PolicyNotice.AdvanceFanout(ctx, older.ID, legal.Lease{Token: "stale", Until: now.Add(2 * time.Minute)}, "u1", legal.FanoutOpen)
	require.NoError(t, err)
	assert.False(t, ok, "a stale token writes nothing")
	renewed := legal.Lease{Token: "a", Until: now.Add(3 * time.Minute)}
	ok, err = c.PolicyNotice.AdvanceFanout(ctx, older.ID, renewed, "u1", legal.FanoutOpen)
	require.NoError(t, err)
	assert.True(t, ok)

	got, err = c.PolicyNotice.ClaimFanout(ctx, now.Add(2*time.Minute), legal.Lease{Token: "d", Until: now.Add(4 * time.Minute)})
	require.NoError(t, err)
	require.NotNil(t, got, "the newer notice's lease expired at its until")
	assert.Equal(t, newer.ID, got.ID)
	ok, err = c.PolicyNotice.AdvanceFanout(ctx, newer.ID, b, "w", legal.FanoutOpen)
	require.NoError(t, err)
	assert.False(t, ok, "the first holder lost the notice when its lease expired")

	got, err = c.PolicyNotice.ClaimFanout(ctx, now.Add(3*time.Minute), legal.Lease{Token: "e", Until: now.Add(5 * time.Minute)})
	require.NoError(t, err)
	require.NotNil(t, got, "the renewed lease ends at its new until")
	assert.Equal(t, older.ID, got.ID)
	assert.Equal(t, "u1", got.Fanout.Cursor, "the cursor survives a lost lease")

	ok, err = c.PolicyNotice.AdvanceFanout(ctx, older.ID, legal.Lease{Token: "e", Until: now.Add(5 * time.Minute)}, "u9", legal.FanoutComplete)
	require.NoError(t, err)
	assert.True(t, ok)
	require.NoError(t, c.PolicyNotice.ReleaseFanout(ctx, newer.ID, "d"))

	list, err := c.PolicyNotice.FindByRef(ctx, tos("2.0.0"))
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, legal.Fanout{Cursor: "u9", End: legal.FanoutComplete}, list[0].Fanout, "an ended notice drops its lease")
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

func testPolicyNoticeReopenWithdrawnOrCovered(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	now := timeFixed()
	end := func(n legal.Notice, e legal.FanoutEnd) {
		t.Helper()
		lease := legal.Lease{Token: "t", Until: now.Add(time.Minute)}
		got, err := c.PolicyNotice.ClaimFanout(ctx, now, lease)
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Equal(t, n.ID, got.ID)
		ok, err := c.PolicyNotice.AdvanceFanout(ctx, n.ID, lease, "u5", e)
		require.NoError(t, err)
		require.True(t, ok)
	}
	withdrawn := notice(tos("2.0.0"), legal.AudienceAll, now)
	complete := notice(tos("2.0.0"), legal.AudienceOwners, now.Add(time.Second))
	covered := notice(tos("3.0.0"), legal.AudienceOwners, now.Add(2*time.Second))
	for _, n := range []legal.Notice{withdrawn, complete, covered} {
		_, _, err := c.PolicyNotice.Create(ctx, n)
		require.NoError(t, err)
	}
	end(withdrawn, legal.FanoutWithdrawn)
	end(complete, legal.FanoutComplete)
	end(covered, legal.FanoutCovered)

	got, outcome, err := c.PolicyNotice.Create(ctx, notice(tos("2.0.0"), legal.AudienceAll, now.Add(time.Hour)))
	require.NoError(t, err)
	assert.Equal(t, legal.NoticeReopened, outcome)
	assert.Equal(t, withdrawn.ID, got.ID)
	assert.Equal(t, legal.Fanout{}, got.Fanout, "a reopened notice starts over")
	_, outcome, err = c.PolicyNotice.Create(ctx, notice(tos("2.0.0"), legal.AudienceAll, now.Add(time.Hour)))
	require.NoError(t, err)
	assert.Equal(t, legal.NoticeExisting, outcome, "an open notice is not reopened")
	got, outcome, err = c.PolicyNotice.Create(ctx, notice(tos("2.0.0"), legal.AudienceOwners, now.Add(time.Hour)))
	require.NoError(t, err)
	assert.Equal(t, legal.NoticeExisting, outcome, "a complete notice is not reopened")
	assert.Equal(t, legal.Fanout{Cursor: "u5", End: legal.FanoutComplete}, got.Fanout)
	got, outcome, err = c.PolicyNotice.Create(ctx, notice(tos("3.0.0"), legal.AudienceOwners, now.Add(time.Hour)))
	require.NoError(t, err)
	assert.Equal(t, legal.NoticeReopened, outcome, "a covered notice reopens")
	assert.Equal(t, covered.ID, got.ID)
	assert.Equal(t, legal.Fanout{}, got.Fanout)

	claimed, err := c.PolicyNotice.ClaimFanout(ctx, now, legal.Lease{Token: "u", Until: now.Add(time.Minute)})
	require.NoError(t, err)
	require.NotNil(t, claimed)
	assert.Equal(t, withdrawn.ID, claimed.ID, "a reopened notice is claimed again")
}

func testPolicyNoticeWithdraw(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	now := timeFixed()
	open := notice(tos("2.0.0"), legal.AudienceAll, now)
	complete := notice(tos("2.0.0"), legal.AudienceOwners, now.Add(time.Second))
	covered := notice(tos("3.0.0"), legal.AudienceAll, now.Add(2*time.Second))
	withdrawn := notice(tos("3.0.0"), legal.AudienceOwners, now.Add(3*time.Second))
	for _, n := range []legal.Notice{open, complete, covered, withdrawn} {
		_, _, err := c.PolicyNotice.Create(ctx, n)
		require.NoError(t, err)
	}
	leases := map[id.PolicyNoticeID]legal.Lease{}
	for _, e := range []legal.FanoutEnd{legal.FanoutOpen, legal.FanoutComplete, legal.FanoutCovered, legal.FanoutWithdrawn} {
		lease := legal.Lease{Token: "t" + string(e), Until: now.Add(time.Minute)}
		got, err := c.PolicyNotice.ClaimFanout(ctx, now, lease)
		require.NoError(t, err)
		require.NotNil(t, got)
		ok, err := c.PolicyNotice.AdvanceFanout(ctx, got.ID, lease, "u5", e)
		require.NoError(t, err)
		require.True(t, ok)
		leases[got.ID] = lease
	}

	for range 2 {
		for _, n := range []legal.Notice{open, complete, covered, withdrawn} {
			require.NoError(t, c.PolicyNotice.Withdraw(ctx, n.ID))
		}
		require.NoError(t, c.PolicyNotice.Withdraw(ctx, id.NewPolicyNoticeID()), "an unknown notice is no error")
	}

	ends := map[id.PolicyNoticeID]legal.Fanout{}
	for _, r := range []legal.Ref{tos("2.0.0"), tos("3.0.0")} {
		list, err := c.PolicyNotice.FindByRef(ctx, r)
		require.NoError(t, err)
		for _, n := range list {
			ends[n.ID] = n.Fanout
		}
	}
	assert.Equal(t, legal.Fanout{Cursor: "u5", End: legal.FanoutWithdrawn}, ends[open.ID], "an open notice is withdrawn and drops its lease")
	assert.Equal(t, legal.Fanout{Cursor: "u5", End: legal.FanoutWithdrawn}, ends[complete.ID], "a complete notice is withdrawn")
	assert.Equal(t, legal.Fanout{Cursor: "u5", End: legal.FanoutCovered}, ends[covered.ID], "a covered notice stays covered")
	assert.Equal(t, legal.Fanout{Cursor: "u5", End: legal.FanoutWithdrawn}, ends[withdrawn.ID])

	ok, err := c.PolicyNotice.AdvanceFanout(ctx, open.ID, leases[open.ID], "u9", legal.FanoutComplete)
	require.NoError(t, err)
	assert.False(t, ok, "the fan-out that held the open notice lost it")
	got, err := c.PolicyNotice.ClaimFanout(ctx, now, legal.Lease{Token: "x", Until: now.Add(time.Minute)})
	require.NoError(t, err)
	assert.Nil(t, got, "a withdrawn notice is not claimed")

	got2, outcome, err := c.PolicyNotice.Create(ctx, notice(tos("2.0.0"), legal.AudienceOwners, now.Add(time.Hour)))
	require.NoError(t, err)
	assert.Equal(t, legal.NoticeReopened, outcome, "a withdrawn complete notice reopens")
	assert.Equal(t, complete.ID, got2.ID)
	assert.Equal(t, legal.Fanout{}, got2.Fanout)
}

func testOutboxRemoveFailedByNotice(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	now := timeFixed()
	n, other := id.NewPolicyNoticeID(), id.NewPolicyNoticeID()
	enqueue := func(k string, nid id.PolicyNoticeID) {
		t.Helper()
		subject := outbox.PolicyNoticeSubject{Bump: legal.BumpMajor, Notice: nid, Ref: tos("2.0.0")}.Subject()
		r, err := outbox.New(outbox.Key(k), testKind, subject, outbox.ToUser(id.NewUserID()), now)
		require.NoError(t, err)
		require.NoError(t, c.Outbox.Enqueue(ctx, []outbox.Record{r}))
	}
	finish := func(k string, tr outbox.Transition) {
		t.Helper()
		cl := claim("t-"+k, now.Add(time.Minute))
		rec, err := c.Outbox.ClaimDue(ctx, now, cl)
		require.NoError(t, err)
		require.NotNil(t, rec)
		require.Equal(t, outbox.Key(k), rec.Key)
		ok, err := c.Outbox.Complete(ctx, rec.Key, "t-"+k, tr, now)
		require.NoError(t, err)
		require.True(t, ok)
	}
	withdrawn := outbox.Transition{To: outbox.StateFailed, Reason: outbox.ReasonVersionNotUpcoming}
	for _, k := range []string{"withdrawn", "gone", "sent", "other"} {
		nid := n
		if k == "other" {
			nid = other
		}
		enqueue(k, nid)
		switch k {
		case "gone":
			finish(k, outbox.Transition{To: outbox.StateFailed, Reason: outbox.ReasonUserGone})
		case "sent":
			finish(k, outbox.Transition{To: outbox.StateSent})
		default:
			finish(k, withdrawn)
		}
	}
	enqueue("pending", n)

	require.NoError(t, c.Outbox.RemoveFailedByNotice(ctx, n, outbox.ReasonVersionNotUpcoming))
	_, err := c.Outbox.FindByKey(ctx, "withdrawn")
	assert.Error(t, err, "the notice's withdrawn record is deleted")
	for _, k := range []outbox.Key{"gone", "sent", "other", "pending"} {
		_, err := c.Outbox.FindByKey(ctx, k)
		assert.NoError(t, err, "%s is kept", k)
	}
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
	deactivated := newUser(t, "deactivated", "deactivated@example.com")
	deactivated.Deactivate()
	require.NoError(t, c.User.Save(ctx, deactivated))
	w3 := org(map[id.UserID]workspace.Member{o3: member(role.RoleOwner, false), deactivated.ID(): member(role.RoleOwner, false)}, nil)
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
	assert.Equal(t, want, got, "enabled, active owners of non-personal, non-deleted workspaces")

	owners, last, err := c.Workspace.FindOrgOwnersAfter(ctx, &orgs[1], 5)
	require.NoError(t, err)
	require.NotNil(t, last)
	assert.Equal(t, orgs[2], *last)
	assert.ElementsMatch(t, ownersOf[orgs[2]], owners)
}

func testAudienceOrgOwnersAmong(t *testing.T, nc Factory) {
	c, _, done := nc(t)
	defer done()
	ctx := context.Background()
	save := func(b *workspace.Builder) {
		t.Helper()
		ws, err := b.Build()
		require.NoError(t, err)
		require.NoError(t, c.Workspace.Save(ctx, ws))
	}
	owner, twice, writer, disabled, personal, deletedOwner, stranger := id.NewUserID(), id.NewUserID(), id.NewUserID(),
		id.NewUserID(), id.NewUserID(), id.NewUserID(), id.NewUserID()
	deletedAt := timeFixed()
	save(workspace.New().NewID().Name("a").Members(map[id.UserID]workspace.Member{
		owner: {Role: role.RoleOwner}, twice: {Role: role.RoleOwner}, writer: {Role: role.RoleWriter}, disabled: {Role: role.RoleOwner, Disabled: true},
	}))
	save(workspace.New().NewID().Name("b").Members(map[id.UserID]workspace.Member{twice: {Role: role.RoleOwner}}))
	save(workspace.New().NewID().Name("p").Personal(true).Members(map[id.UserID]workspace.Member{personal: {Role: role.RoleOwner}}))
	save(workspace.New().NewID().Name("d").DeletedAt(&deletedAt).Members(map[id.UserID]workspace.Member{deletedOwner: {Role: role.RoleOwner}}))

	got, err := c.Workspace.FindOrgOwnersAmong(ctx, user.IDList{owner, twice, writer, disabled, personal, deletedOwner, stranger})
	require.NoError(t, err)
	assert.ElementsMatch(t, user.IDList{owner, twice}, got, "each enabled owner of a live organization, once")

	got, err = c.Workspace.FindOrgOwnersAmong(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, got)
}
