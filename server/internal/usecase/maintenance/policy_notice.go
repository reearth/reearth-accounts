package maintenance

import (
	"context"
	"errors"
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/applog"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/legal"
	"github.com/reearth/reearth-accounts/server/pkg/mail"
	"github.com/reearth/reearth-accounts/server/pkg/outbox"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/rerror"
)

const (
	FanoutLease = 2 * time.Minute
	FanoutPage  = 500
)

type PolicyNoticeRepos struct {
	Acceptances legal.AcceptanceRepo
	Notices     legal.NoticeRepo
	Outbox      outbox.Repo
	Users       user.Repo
	Versions    legal.VersionRepo
	Workspaces  workspace.Repo
}

// PolicyNoticeFanout queues one record per user of each notice's audience,
// a page at a time, while it holds the notice's lease.
func PolicyNoticeFanout(r PolicyNoticeRepos, page int) Producer {
	return &policyNoticeFanout{now: func() time.Time { return time.Now().UTC() }, page: page, repos: r, token: newToken}
}

// PolicyNoticeResolver builds a notice when it is sent, from the user's
// current address and language and the version as it is stored then.
func PolicyNoticeResolver(r PolicyNoticeRepos) Resolver {
	return &policyNoticeResolver{now: func() time.Time { return time.Now().UTC() }, repos: r}
}

type policyNoticeFanout struct {
	now   func() time.Time
	page  int
	repos PolicyNoticeRepos
	token func() string
}

func (*policyNoticeFanout) Name() string { return "policy_notice_fanout" }

func (f *policyNoticeFanout) Run(ctx context.Context, now time.Time) (int, error) {
	queued := 0
	for ctx.Err() == nil {
		lease := legal.Lease{Token: f.token(), Until: now.Add(FanoutLease)}
		n, err := f.repos.Notices.ClaimFanout(ctx, now, lease)
		if err != nil || n == nil {
			return queued, quiet(ctx, err)
		}
		q, err := f.fanout(ctx, *n, lease)
		queued += q
		if err != nil {
			rctx, cancel := detached(ctx, resultTimeout)
			_ = f.repos.Notices.ReleaseFanout(rctx, n.ID, lease.Token)
			cancel()
			return queued, quiet(ctx, err)
		}
		now = f.now()
	}
	return queued, nil
}

// fanout pages the audience of n until it is done, the lease is lost or ctx
// ends. A notice whose version is no longer upcoming queues nothing more.
func (f *policyNoticeFanout) fanout(ctx context.Context, n legal.Notice, lease legal.Lease) (int, error) {
	heads, err := f.repos.Versions.FindHeads(ctx)
	if err != nil {
		return 0, err
	}
	cat := legal.NewCatalog(heads)
	cursor, queued := n.Fanout.Cursor, 0
	for {
		up := cat.Upcoming(n.Ref.Type, f.now())
		if up == nil || up.Ref != n.Ref {
			_, err := f.repos.Notices.AdvanceFanout(ctx, n.ID, lease, cursor, true)
			return queued, err
		}
		users, next, err := f.audience(ctx, n.Audience, cursor)
		if err != nil {
			return queued, err
		}
		if n.Bump == legal.BumpMajor {
			if users, err = f.notHolding(ctx, cat, users, n.Ref); err != nil {
				return queued, err
			}
		}
		if err := f.enqueue(ctx, n, users); err != nil {
			return queued, err
		}
		queued += len(users)
		lease.Until = f.now().Add(FanoutLease)
		held, err := f.repos.Notices.AdvanceFanout(ctx, n.ID, lease, next, next == "")
		if err != nil || next == "" {
			return queued, err
		}
		if !held {
			applog.Warn(ctx, "policy notice fanout lost its lease", applog.NoticeID(n.ID))
			return queued, nil
		}
		cursor = next
		if ctx.Err() != nil {
			return queued, ctx.Err()
		}
	}
}

// audience returns the next page of users after cursor, and the cursor after
// it, which is empty when the audience is exhausted.
func (f *policyNoticeFanout) audience(ctx context.Context, a legal.Audience, cursor string) (user.IDList, string, error) {
	switch a {
	case legal.AudienceAll:
		after, err := cursorID(cursor, user.IDFrom)
		if err != nil {
			return nil, "", err
		}
		ids, err := f.repos.Users.FindActiveIDsWithEmailAfter(ctx, after, f.page)
		if err != nil || len(ids) == 0 {
			return nil, "", err
		}
		return ids, ids[len(ids)-1].String(), nil
	case legal.AudienceOwners:
		after, err := cursorID(cursor, workspace.IDFrom)
		if err != nil {
			return nil, "", err
		}
		owners, last, err := f.repos.Workspaces.FindOrgOwnersAfter(ctx, after, f.page)
		if err != nil || last == nil {
			return nil, "", err
		}
		return owners, last.String(), nil
	}
	return nil, "", errors.New("policy notice: unknown audience " + string(a))
}

func (f *policyNoticeFanout) enqueue(ctx context.Context, n legal.Notice, users user.IDList) error {
	if len(users) == 0 {
		return nil
	}
	subject := outbox.PolicyNoticeSubject{Bump: n.Bump, Notice: n.ID, Ref: n.Ref}.Subject()
	now := f.now()
	rs := make([]outbox.Record, 0, len(users))
	for _, u := range users {
		r, err := outbox.New(outbox.PolicyNoticeKey(n.Ref, u), mail.KindPolicyNotice, subject, outbox.ToUser(u), now)
		if err != nil {
			return err
		}
		rs = append(rs, r)
	}
	return f.repos.Outbox.Enqueue(ctx, rs)
}

func (f *policyNoticeFanout) notHolding(ctx context.Context, cat legal.Catalog, users user.IDList, r legal.Ref) (user.IDList, error) {
	if len(users) == 0 {
		return users, nil
	}
	as, err := f.repos.Acceptances.FindByUsers(ctx, users, r.Type)
	if err != nil {
		return nil, err
	}
	byUser := map[id.UserID][]legal.Acceptance{}
	for _, a := range as {
		byUser[a.UserID] = append(byUser[a.UserID], a)
	}
	res := make(user.IDList, 0, len(users))
	for _, u := range users {
		if !legal.HoldsAtLeast(cat, byUser[u], r) {
			res = append(res, u)
		}
	}
	return res, nil
}

type policyNoticeResolver struct {
	now   func() time.Time
	repos PolicyNoticeRepos
}

func (r *policyNoticeResolver) Resolve(ctx context.Context, rec outbox.Record) (*mail.Envelope, outbox.Reason, error) {
	s, err := outbox.ParsePolicyNoticeSubject(rec.Subject)
	if err != nil {
		return nil, "", err
	}
	uid, ok := rec.Recipient.User()
	if !ok {
		return nil, "", errors.New("policy notice: recipient is not a user")
	}
	u, err := r.repos.Users.FindByID(ctx, uid)
	if errors.Is(err, rerror.ErrNotFound) || (err == nil && u.IsDeleted()) {
		return nil, outbox.ReasonUserGone, nil
	}
	if err != nil {
		return nil, "", err
	}
	if u.Email() == "" {
		return nil, outbox.ReasonNoAddress, nil
	}
	heads, err := r.repos.Versions.FindHeads(ctx)
	if err != nil {
		return nil, "", err
	}
	cat := legal.NewCatalog(heads)
	up := cat.Upcoming(s.Ref.Type, r.now())
	if up == nil || up.Ref != s.Ref {
		return nil, outbox.ReasonVersionNotUpcoming, nil
	}
	as, err := r.repos.Acceptances.FindByUser(ctx, uid)
	if err != nil {
		return nil, "", err
	}
	if legal.HoldsAtLeast(cat, as, s.Ref) {
		return nil, outbox.ReasonAlreadyAccepted, nil
	}
	lang := u.Metadata().Lang()
	msg, err := mail.Render(lang, mail.PolicyNoticeData{
		ActionURL:     up.URL.Pick(lang.String()),
		EffectiveDate: up.EffectiveAt,
		Major:         s.Bump == legal.BumpMajor,
		PolicyTitle:   up.Title.Pick(lang.String()),
		Version:       up.Version.String(),
	})
	if err != nil {
		return nil, "", err
	}
	return &mail.Envelope{To: mail.ToUser(uid, u.Email(), u.Name()), Message: msg}, "", nil
}

func cursorID[T any](cursor string, parse func(string) (T, error)) (*T, error) {
	if cursor == "" {
		return nil, nil
	}
	v, err := parse(cursor)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// quiet drops the error a write returns because the pass budget ended, which
// is how every long fan-out stops.
func quiet(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return nil
	}
	return err
}
