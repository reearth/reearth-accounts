package maintenance

import (
	"context"
	"testing"
	"time"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/legal"
	"github.com/reearth/reearth-accounts/server/pkg/mail"
	"github.com/reearth/reearth-accounts/server/pkg/outbox"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

var (
	tos1 = legal.Ref{Type: "tos", Version: legal.MustParseVersion("1.0.0")}
	tos2 = legal.Ref{Type: "tos", Version: legal.MustParseVersion("2.0.0")}
)

type noticeFixture struct {
	clock  time.Time
	fanout *policyNoticeFanout
	mail   *fakeMail
	repos  *repo.Container
	runner *Runner
}

func newNoticeFixture(t *testing.T, page int) *noticeFixture {
	t.Helper()
	c := memory.New()
	f := &noticeFixture{clock: t0, mail: &fakeMail{}, repos: c}
	now := func() time.Time { return f.clock }
	r := PolicyNoticeRepos{
		Acceptances: c.PolicyAcceptance, Notices: c.PolicyNotice, Outbox: c.Outbox,
		Users: c.User, Versions: c.PolicyVersion, Workspaces: c.Workspace,
	}
	f.fanout = &policyNoticeFanout{now: now, page: page, repos: r, token: newToken}
	resolvers := map[mail.Kind]Resolver{mail.KindPolicyNotice: &policyNoticeResolver{now: now, repos: r}}
	f.runner = NewRunner(defaultConfig(), c.Outbox, f.mail, resolvers, []Producer{f.fanout})
	f.runner.now = now
	f.runner.jitter = func(d time.Duration) time.Duration { return d }
	f.publish(t, legal.Entry{Ref: tos1, EffectiveAt: t0.Add(-30 * 24 * time.Hour), Required: true, Title: legal.LocalizedText{"en": "Terms"}},
		legal.Entry{Ref: tos2, EffectiveAt: t0.Add(30 * 24 * time.Hour), Required: true,
			Title: legal.LocalizedText{"en": "Terms of Service", "ja": "利用規約"},
			URL:   legal.LocalizedText{"en": "https://example.com/en/tos", "ja": "https://example.com/ja/tos"}})
	return f
}

func (f *noticeFixture) publish(t *testing.T, entries ...legal.Entry) {
	t.Helper()
	f.clock = f.clock.Add(time.Second)
	at := f.clock
	for _, e := range entries {
		_, err := legal.Apply(context.Background(), f.repos.PolicyVersion, legal.Manifest{Timestamp: at, Entries: []legal.Entry{e}}, func() time.Time { return at })
		require.NoError(t, err)
	}
}

func (f *noticeFixture) user(t *testing.T, name, lang string) *user.User {
	t.Helper()
	m := user.NewMetadata()
	m.LangFrom(lang)
	u, err := user.New().NewID().Name(name).Email(name + "@example.com").Workspace(id.NewWorkspaceID()).Metadata(m).Build()
	require.NoError(t, err)
	require.NoError(t, f.repos.User.Save(context.Background(), u))
	return u
}

func (f *noticeFixture) notice(t *testing.T, a legal.Audience) legal.Notice {
	t.Helper()
	n, _ := f.request(t, a)
	return n
}

func (f *noticeFixture) request(t *testing.T, a legal.Audience) (legal.Notice, legal.CreateOutcome) {
	t.Helper()
	heads, err := f.repos.PolicyVersion.FindHeads(context.Background())
	require.NoError(t, err)
	n, err := legal.NewNotice(legal.NewCatalog(heads), "tos", a, id.NewAdminUserID(), f.clock)
	require.NoError(t, err)
	stored, outcome, err := f.repos.PolicyNotice.Create(context.Background(), n)
	require.NoError(t, err)
	return stored, outcome
}

func (f *noticeFixture) stored(t *testing.T, n legal.Notice) legal.Notice {
	t.Helper()
	ns, err := f.repos.PolicyNotice.FindByRef(context.Background(), n.Ref)
	require.NoError(t, err)
	for _, s := range ns {
		if s.ID == n.ID {
			return s
		}
	}
	require.FailNow(t, "notice not stored")
	return legal.Notice{}
}

func (f *noticeFixture) org(t *testing.T, owner user.ID) {
	t.Helper()
	ws, err := workspace.New().NewID().Name("org").Members(map[id.UserID]workspace.Member{owner: {Role: role.RoleOwner}}).Build()
	require.NoError(t, err)
	require.NoError(t, f.repos.Workspace.Save(context.Background(), ws))
}

func (f *noticeFixture) mailsTo(u user.ID) int {
	n := 0
	for _, e := range f.mail.got {
		if *e.To.UserID() == u {
			n++
		}
	}
	return n
}

func (f *noticeFixture) accept(t *testing.T, u user.ID, r legal.Ref) {
	t.Helper()
	require.NoError(t, f.repos.PolicyAcceptance.InsertIfAbsent(context.Background(), []legal.Acceptance{{AcceptedAt: f.clock, Ref: r, UserID: u}}))
}

func (f *noticeFixture) record(t *testing.T, u user.ID) *outbox.Record {
	t.Helper()
	r, err := f.repos.Outbox.FindByKey(context.Background(), outbox.PolicyNoticeKey(tos2, u))
	if err != nil {
		return nil
	}
	return r
}

func (f *noticeFixture) counts(t *testing.T, n legal.Notice) map[outbox.State]int {
	t.Helper()
	c, err := f.repos.Outbox.CountByNotice(context.Background(), n.ID)
	require.NoError(t, err)
	return c
}

func (f *noticeFixture) pass(t *testing.T) Report {
	t.Helper()
	rep, err := f.runner.RunPass(context.Background())
	require.NoError(t, err)
	return rep
}

func TestPolicyNotice_QueuesTheAudienceAndSendsInTheUsersLanguage(t *testing.T) {
	f := newNoticeFixture(t, 2)
	en := f.user(t, "en", "en")
	ja := f.user(t, "ja", "ja")
	other := f.user(t, "other", "")
	accepted := f.user(t, "accepted", "en")
	f.accept(t, accepted.ID(), tos2)
	gone := f.user(t, "gone", "en")
	gone.Deactivate()
	require.NoError(t, f.repos.User.Save(context.Background(), gone))
	n := f.notice(t, legal.AudienceAll)

	rep := f.pass(t)
	assert.Equal(t, 3, rep.Queued)
	assert.Equal(t, 3, rep.Sent)
	for _, u := range []*user.User{en, ja, other} {
		r := f.record(t, u.ID())
		require.NotNil(t, r, u.Name())
		assert.Equal(t, outbox.StateSent, r.State)
	}
	assert.Nil(t, f.record(t, accepted.ID()), "a major notice skips a user who accepted the version")
	assert.Nil(t, f.record(t, gone.ID()), "a deleted user is not in the audience")

	byUser := map[id.UserID]mail.Envelope{}
	for _, e := range f.mail.got {
		byUser[*e.To.UserID()] = e
	}
	require.Len(t, byUser, 3)
	assert.Equal(t, "【Re:Earth】利用規約改定のお知らせ", byUser[ja.ID()].Message.Subject())
	assert.Contains(t, byUser[ja.ID()].Message.Text(), "https://example.com/ja/tos")
	assert.Equal(t, "[Re:Earth] Changes to the Terms of Service", byUser[other.ID()].Message.Subject(), "an unset language gets English")
	assert.Equal(t, "en@example.com", byUser[en.ID()].To.Address(), "the address is read at send time")

	stored, err := f.repos.PolicyNotice.FindByRef(context.Background(), tos2)
	require.NoError(t, err)
	require.Len(t, stored, 1)
	assert.Equal(t, legal.FanoutComplete, stored[0].Fanout.End)
	assert.Nil(t, stored[0].Fanout.Lease)
	want := outbox.EmptyCounts()
	want[outbox.StateSent] = 3
	assert.Equal(t, want, f.counts(t, n))
}

func TestPolicyNotice_RepeatedRequestQueuesNothingNew(t *testing.T) {
	f := newNoticeFixture(t, 10)
	u := f.user(t, "u", "en")
	owner := f.user(t, "owner", "en")
	ws, err := workspace.New().NewID().Name("org").Members(map[id.UserID]workspace.Member{owner.ID(): {Role: role.RoleOwner}}).Build()
	require.NoError(t, err)
	require.NoError(t, f.repos.Workspace.Save(context.Background(), ws))
	first := f.notice(t, legal.AudienceAll)
	f.pass(t)
	require.Equal(t, 2, f.mail.count())

	again := f.notice(t, legal.AudienceAll)
	assert.Equal(t, first.ID, again.ID, "the same request returns the stored notice")
	rep := f.pass(t)
	assert.Equal(t, 0, rep.Queued)

	owners := f.notice(t, legal.AudienceOwners)
	f.pass(t)
	assert.Equal(t, 2, f.mail.count(), "an owner already mailed for this version gets no second mail")
	assert.Equal(t, outbox.StateSent, f.record(t, u.ID()).State)
	assert.Equal(t, outbox.EmptyCounts(), f.counts(t, owners), "the records stay counted under the first notice")
	assert.Equal(t, legal.FanoutCovered, f.stored(t, owners).Fanout.End)
}

func TestPolicyNotice_OwnersAudience(t *testing.T) {
	f := newNoticeFixture(t, 1)
	owner := f.user(t, "owner", "en")
	writer := f.user(t, "writer", "en")
	personal := f.user(t, "personal", "en")
	for _, b := range []*workspace.Builder{
		workspace.New().NewID().Name("org").Members(map[id.UserID]workspace.Member{owner.ID(): {Role: role.RoleOwner}, writer.ID(): {Role: role.RoleWriter}}),
		workspace.New().NewID().Name("org2").Members(map[id.UserID]workspace.Member{owner.ID(): {Role: role.RoleOwner}}),
		workspace.New().NewID().Name("p").Personal(true).Members(map[id.UserID]workspace.Member{personal.ID(): {Role: role.RoleOwner}}),
	} {
		ws, err := b.Build()
		require.NoError(t, err)
		require.NoError(t, f.repos.Workspace.Save(context.Background(), ws))
	}
	f.notice(t, legal.AudienceOwners)

	f.pass(t)
	assert.Equal(t, 1, f.mail.count(), "an owner of two organizations gets one mail")
	assert.NotNil(t, f.record(t, owner.ID()))
	assert.Nil(t, f.record(t, writer.ID()))
	assert.Nil(t, f.record(t, personal.ID()), "a personal workspace's owner is not in the audience")
}

func TestPolicyNotice_FanoutResumesFromItsCursor(t *testing.T) {
	f := newNoticeFixture(t, 2)
	var users []*user.User
	for _, name := range []string{"a", "b", "c", "d", "e"} {
		users = append(users, f.user(t, name, "en"))
	}
	n := f.notice(t, legal.AudienceAll)
	f.mail.noSender = true

	ctx, cancel := context.WithCancel(context.Background())
	pages := 0
	f.fanout.repos.Users = pageCounter{Repo: f.repos.User, onPage: func() {
		if pages++; pages == 1 {
			cancel()
		}
	}}
	queued, err := f.fanout.Run(ctx, f.clock)
	require.NoError(t, err, "a budget that ends mid fan-out is not an error")
	assert.Equal(t, 2, queued)
	stored, err := f.repos.PolicyNotice.FindByRef(context.Background(), tos2)
	require.NoError(t, err)
	assert.False(t, stored[0].Fanout.Done())
	assert.Nil(t, stored[0].Fanout.Lease, "the lease is released, so the next pass resumes at once")

	queued, err = f.fanout.Run(context.Background(), f.clock)
	require.NoError(t, err)
	assert.Equal(t, 3, queued, "the next pass starts after the cursor")
	assert.Equal(t, 4, pages, "pages of two, two and one, then an empty page")
	for _, u := range users {
		assert.NotNil(t, f.record(t, u.ID()), u.Name())
	}
	want := outbox.EmptyCounts()
	want[outbox.StatePending] = 5
	assert.Equal(t, want, f.counts(t, n), "without a provider every record waits")
}

func TestPolicyNotice_NotSentWhenItNoLongerApplies(t *testing.T) {
	f := newNoticeFixture(t, 10)
	accepts := f.user(t, "accepts", "en")
	leaves := f.user(t, "leaves", "en")
	removed := f.user(t, "removed", "en")
	stays := f.user(t, "stays", "en")
	f.notice(t, legal.AudienceAll)
	f.mail.noSender = true
	f.pass(t)

	f.accept(t, accepts.ID(), tos2)
	leaves.Deactivate()
	require.NoError(t, f.repos.User.Save(context.Background(), leaves))
	require.NoError(t, f.repos.User.Remove(context.Background(), removed.ID()))
	f.mail.noSender = false
	f.pass(t)

	assert.Equal(t, outbox.ReasonAlreadyAccepted, f.record(t, accepts.ID()).Reason)
	assert.Equal(t, outbox.ReasonUserGone, f.record(t, leaves.ID()).Reason)
	assert.Equal(t, outbox.ReasonUserGone, f.record(t, removed.ID()).Reason)
	assert.Equal(t, outbox.StateSent, f.record(t, stays.ID()).State)
	assert.Equal(t, 1, f.mail.count())
}

func TestPolicyNotice_WithdrawnVersion(t *testing.T) {
	f := newNoticeFixture(t, 1)
	a := f.user(t, "a", "en")
	b := f.user(t, "b", "en")
	n := f.notice(t, legal.AudienceAll)
	f.mail.noSender = true
	_, err := f.fanout.Run(cancelAfterPages(t, f, 1), f.clock)
	require.NoError(t, err)
	require.NotNil(t, f.record(t, a.ID()))

	f.publish(t, legal.Entry{Ref: tos1, EffectiveAt: t0.Add(-30 * 24 * time.Hour), Required: true, Title: legal.LocalizedText{"en": "Terms"}})
	f.mail.noSender = false
	rep := f.pass(t)

	assert.Equal(t, 0, rep.Queued, "a withdrawn version queues nothing more")
	assert.Nil(t, f.record(t, b.ID()))
	r := f.record(t, a.ID())
	assert.Equal(t, outbox.StateFailed, r.State)
	assert.Equal(t, outbox.ReasonVersionNotUpcoming, r.Reason)
	assert.Equal(t, 0, f.mail.count())
	stored, err := f.repos.PolicyNotice.FindByRef(context.Background(), n.Ref)
	require.NoError(t, err)
	assert.Equal(t, legal.FanoutWithdrawn, stored[0].Fanout.End)
}

func TestPolicyNotice_MinorNoticeDoesNotSkipAtFanout(t *testing.T) {
	f := newNoticeFixture(t, 10)
	minor := legal.Ref{Type: "tos", Version: legal.MustParseVersion("1.1.0")}
	f.publish(t, legal.Entry{Ref: minor, EffectiveAt: t0.Add(30 * 24 * time.Hour), Required: true, Title: legal.LocalizedText{"en": "Terms"}})
	u := f.user(t, "u", "ja")
	f.accept(t, u.ID(), minor)
	n := f.notice(t, legal.AudienceAll)
	require.Equal(t, legal.BumpMinor, n.Bump)

	rep := f.pass(t)
	assert.Equal(t, 1, rep.Queued)
	r, err := f.repos.Outbox.FindByKey(context.Background(), outbox.PolicyNoticeKey(minor, u.ID()))
	require.NoError(t, err)
	assert.Equal(t, outbox.ReasonAlreadyAccepted, r.Reason, "the resolver still skips a user who accepted it")
}

func TestPolicyNotice_ResolverRendersMinorWithoutURL(t *testing.T) {
	f := newNoticeFixture(t, 10)
	minor := legal.Ref{Type: "privacy", Version: legal.MustParseVersion("1.1.0")}
	f.publish(t, legal.Entry{Ref: legal.Ref{Type: "privacy", Version: legal.MustParseVersion("1.0.0")}, EffectiveAt: t0.Add(-time.Hour), Required: true},
		legal.Entry{Ref: minor, EffectiveAt: t0.Add(time.Hour), Required: true, Title: legal.LocalizedText{"en": "Privacy Policy"}})
	u := f.user(t, "u", "en")
	res := f.runner.resolvers[mail.KindPolicyNotice]
	rec, err := outbox.New(outbox.PolicyNoticeKey(minor, u.ID()), mail.KindPolicyNotice,
		outbox.PolicyNoticeSubject{Bump: legal.BumpMinor, Notice: id.NewPolicyNoticeID(), Ref: minor}.Subject(), outbox.ToUser(u.ID()), f.clock)
	require.NoError(t, err)

	env, reason, err := res.Resolve(context.Background(), rec)
	require.NoError(t, err)
	require.NotNil(t, env, reason)
	want, err := mail.Render(language.English, mail.PolicyNoticeData{EffectiveDate: t0.Add(time.Hour), PolicyTitle: "Privacy Policy", Version: "1.1.0"})
	require.NoError(t, err)
	assert.Equal(t, want, env.Message)
}

func TestPolicyNotice_SecondAudienceAfterRetentionSendsNoSecondMail(t *testing.T) {
	tos3 := legal.Ref{Type: "tos", Version: legal.MustParseVersion("3.0.0")}
	for _, tt := range []struct {
		first, second legal.Audience
		end           legal.FanoutEnd
	}{
		{first: legal.AudienceAll, second: legal.AudienceOwners, end: legal.FanoutCovered},
		{first: legal.AudienceOwners, second: legal.AudienceAll, end: legal.FanoutComplete},
	} {
		t.Run(string(tt.first)+" then "+string(tt.second), func(t *testing.T) {
			f := newNoticeFixture(t, 10)
			f.publish(t, legal.Entry{Ref: tos1, EffectiveAt: t0.Add(-30 * 24 * time.Hour), Required: true, Title: legal.LocalizedText{"en": "Terms"}},
				legal.Entry{Ref: tos3, EffectiveAt: t0.Add(90 * 24 * time.Hour), Required: true, Title: legal.LocalizedText{"en": "Terms of Service"}})
			owner := f.user(t, "owner", "en")
			member := f.user(t, "member", "en")
			f.org(t, owner.ID())
			f.notice(t, tt.first)
			f.pass(t)
			require.Equal(t, 1, f.mailsTo(owner.ID()))

			f.clock = f.clock.Add(721 * time.Hour)
			f.pass(t)
			_, err := f.repos.Outbox.FindByKey(context.Background(), outbox.PolicyNoticeKey(tos3, owner.ID()))
			require.Error(t, err, "retention deleted the sent record")

			second := f.notice(t, tt.second)
			f.pass(t)
			assert.Equal(t, 1, f.mailsTo(owner.ID()), "an owner gets one mail per version across both audiences")
			assert.Equal(t, 1, f.mailsTo(member.ID()), "a member gets the all notice")
			assert.Equal(t, tt.end, f.stored(t, second).Fanout.End)
		})
	}
}

func TestPolicyNotice_WithdrawnAndRepublishedVersion(t *testing.T) {
	f := newNoticeFixture(t, 1)
	a := f.user(t, "a", "en")
	b := f.user(t, "b", "en")
	tos2Entry := legal.Entry{Ref: tos2, EffectiveAt: t0.Add(30 * 24 * time.Hour), Required: true, Title: legal.LocalizedText{"en": "Terms of Service"}}
	tos1Entry := legal.Entry{Ref: tos1, EffectiveAt: t0.Add(-30 * 24 * time.Hour), Required: true, Title: legal.LocalizedText{"en": "Terms"}}
	n := f.notice(t, legal.AudienceAll)
	f.mail.noSender = true
	_, err := f.fanout.Run(cancelAfterPages(t, f, 1), f.clock)
	require.NoError(t, err)
	require.NotNil(t, f.record(t, a.ID()))

	f.publish(t, tos1Entry)
	f.mail.noSender = false
	f.pass(t)
	require.Equal(t, outbox.ReasonVersionNotUpcoming, f.record(t, a.ID()).Reason)
	require.Equal(t, legal.FanoutWithdrawn, f.stored(t, n).Fanout.End)

	f.publish(t, tos1Entry, tos2Entry)
	again, outcome := f.request(t, legal.AudienceAll)
	assert.Equal(t, legal.NoticeReopened, outcome)
	assert.Equal(t, n.ID, again.ID)
	rep := f.pass(t)

	assert.Equal(t, 2, rep.Queued, "the reopened notice queues its whole audience")
	for _, u := range []*user.User{a, b} {
		r := f.record(t, u.ID())
		require.NotNil(t, r, u.Name())
		assert.Equal(t, outbox.StateSent, r.State, u.Name())
	}
	assert.Equal(t, 2, f.mail.count())
	assert.Equal(t, legal.FanoutComplete, f.stored(t, n).Fanout.End)
}

func TestPolicyNotice_TitleAndURLFollowTheTemplateLanguage(t *testing.T) {
	f := newNoticeFixture(t, 10)
	f.publish(t, legal.Entry{Ref: tos1, EffectiveAt: t0.Add(-30 * 24 * time.Hour), Required: true, Title: legal.LocalizedText{"en": "Terms"}},
		legal.Entry{Ref: tos2, EffectiveAt: t0.Add(30 * 24 * time.Hour), Required: true,
			Title: legal.LocalizedText{"de": "Nutzungsbedingungen", "en": "Terms of Service", "ja": "利用規約"},
			URL:   legal.LocalizedText{"de": "https://example.com/de/tos", "en": "https://example.com/en/tos", "ja": "https://example.com/ja/tos"}})
	users := map[string]*user.User{}
	for _, l := range []string{"de", "en-GB", "ja-JP", "zz"} {
		users[l] = f.user(t, "u"+l, l)
	}
	f.notice(t, legal.AudienceAll)
	f.pass(t)

	byUser := map[id.UserID]mail.Envelope{}
	for _, e := range f.mail.got {
		byUser[*e.To.UserID()] = e
	}
	for l, want := range map[string]struct{ subject, url string }{
		"de":    {"[Re:Earth] Changes to the Terms of Service", "https://example.com/en/tos"},
		"en-GB": {"[Re:Earth] Changes to the Terms of Service", "https://example.com/en/tos"},
		"ja-JP": {"【Re:Earth】利用規約改定のお知らせ", "https://example.com/ja/tos"},
		"zz":    {"[Re:Earth] Changes to the Terms of Service", "https://example.com/en/tos"},
	} {
		e := byUser[users[l].ID()]
		assert.Equal(t, want.subject, e.Message.Subject(), l)
		assert.Contains(t, e.Message.Text(), want.url, l)
		assert.NotContains(t, e.Message.Text(), "Nutzungsbedingungen", l)
	}
}

type pageCounter struct {
	user.Repo
	onPage func()
}

func (p pageCounter) FindActiveIDsWithEmailAfter(ctx context.Context, after *user.ID, limit int) (user.IDList, error) {
	ids, err := p.Repo.FindActiveIDsWithEmailAfter(ctx, after, limit)
	p.onPage()
	return ids, err
}

func cancelAfterPages(t *testing.T, f *noticeFixture, n int) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	pages := 0
	inner := f.fanout.repos.Users
	f.fanout.repos.Users = pageCounter{Repo: inner, onPage: func() {
		if pages++; pages == n {
			cancel()
		}
	}}
	t.Cleanup(func() { f.fanout.repos.Users = inner })
	return ctx
}
