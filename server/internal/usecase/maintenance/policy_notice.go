package maintenance

import (
	"context"
	"errors"
	"slices"
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

func PolicyNoticeFanout(r PolicyNoticeRepos, page int) Producer {
	return &policyNoticeFanout{now: func() time.Time { return time.Now().UTC() }, page: page, repos: r, token: newToken}
}

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
			return queued, unlessPassEnded(ctx, err)
		}
		q, err := f.fanout(ctx, *n, lease)
		queued += q
		if err != nil {
			rctx, cancel := detached(ctx, resultTimeout)
			_ = f.repos.Notices.ReleaseFanout(rctx, n.ID, lease.Token)
			cancel()
			return queued, unlessPassEnded(ctx, err)
		}
		now = f.now()
	}
	return queued, nil
}

func (f *policyNoticeFanout) fanout(ctx context.Context, n legal.Notice, lease legal.Lease) (int, error) {
	siblings, err := f.repos.Notices.FindByRef(ctx, n.Ref)
	if err != nil {
		return 0, err
	}
	overlap := legal.OverlapOf(n, siblings)
	if overlap == legal.OverlapCovered {
		applog.Info(ctx, "policy notice covered by an earlier notice", applog.NoticeID(n.ID))
		_, err := f.repos.Notices.AdvanceFanout(ctx, n.ID, lease, n.Fanout.Cursor, legal.FanoutCovered)
		return 0, err
	}
	heads, err := f.repos.Versions.FindHeads(ctx)
	if err != nil {
		return 0, err
	}
	cat := legal.NewCatalog(heads)
	cursor, queued := n.Fanout.Cursor, 0
	for {
		up := cat.Upcoming(n.Ref.Type, f.now())
		if up == nil || up.Ref != n.Ref {
			_, err := f.repos.Notices.AdvanceFanout(ctx, n.ID, lease, cursor, legal.FanoutWithdrawn)
			return queued, err
		}
		if cursor == "" {
			// A reopened notice starts over, and the records it or a withdrawn
			// notice of the other audience queued before the withdrawal would
			// keep their keys from being queued again.
			for _, o := range siblings {
				if o.ID != n.ID && o.Fanout.End != legal.FanoutWithdrawn {
					continue
				}
				if err := f.repos.Outbox.RemoveFailedByNotice(ctx, o.ID, outbox.ReasonVersionNotUpcoming); err != nil {
					return queued, err
				}
			}
		}
		page, err := f.audience(ctx, n.Audience, cursor)
		if err != nil {
			return queued, err
		}
		users := page.users
		if n.Bump == legal.BumpMajor {
			if users, err = f.notHolding(ctx, cat, users, n.Ref); err != nil {
				return queued, err
			}
		}
		if overlap == legal.OverlapSkipOwners {
			if users, err = f.notOwners(ctx, users); err != nil {
				return queued, err
			}
		}
		if err := f.enqueue(ctx, n, users); err != nil {
			return queued, err
		}
		queued += len(users)
		end := legal.FanoutOpen
		if page.done {
			end = legal.FanoutComplete
		}
		lease.Until = f.now().Add(FanoutLease)
		held, err := f.repos.Notices.AdvanceFanout(ctx, n.ID, lease, page.next, end)
		if err != nil || page.done {
			return queued, err
		}
		if !held {
			applog.Warn(ctx, "policy notice fanout lost its lease", applog.NoticeID(n.ID))
			return queued, nil
		}
		cursor = page.next
		if ctx.Err() != nil {
			return queued, ctx.Err()
		}
	}
}

type audiencePage struct {
	done  bool
	next  string
	users user.IDList
}

func (f *policyNoticeFanout) audience(ctx context.Context, a legal.Audience, cursor string) (audiencePage, error) {
	switch a {
	case legal.AudienceAll:
		after, err := cursorID(cursor, user.IDFrom)
		if err != nil {
			return audiencePage{}, err
		}
		ids, err := f.repos.Users.FindActiveIDsWithEmailAfter(ctx, after, f.page)
		if err != nil || len(ids) == 0 {
			return audiencePage{done: true}, err
		}
		return audiencePage{next: ids[len(ids)-1].String(), users: ids}, nil
	case legal.AudienceOwners:
		after, err := cursorID(cursor, workspace.IDFrom)
		if err != nil {
			return audiencePage{}, err
		}
		owners, last, err := f.repos.Workspaces.FindOrgOwnersAfter(ctx, after, f.page)
		if err != nil || last == nil {
			return audiencePage{done: true}, err
		}
		return audiencePage{next: last.String(), users: owners}, nil
	}
	return audiencePage{}, errors.New("policy notice: unknown audience " + string(a))
}

func (f *policyNoticeFanout) notOwners(ctx context.Context, users user.IDList) (user.IDList, error) {
	if len(users) == 0 {
		return users, nil
	}
	owners, err := f.repos.Workspaces.FindOrgOwnersAmong(ctx, users)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(users, func(u user.ID) bool { return owners.Has(u) }), nil
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
	cat, up, err := r.upcoming(ctx, s.Ref)
	if err != nil {
		return nil, "", err
	}
	if up == nil {
		// The version may have been republished and the notice requested
		// again since the read, and a withdrawal then would end the notice
		// that request left open.
		if cat, up, err = r.upcoming(ctx, s.Ref); err != nil {
			return nil, "", err
		}
	}
	if up == nil {
		if err := r.repos.Notices.Withdraw(ctx, s.Notice); err != nil {
			return nil, "", err
		}
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
	tmpl := string(mail.LangOf(lang))
	msg, err := mail.Render(lang, mail.PolicyNoticeData{
		ActionURL:     up.URL.Pick(tmpl),
		EffectiveDate: up.EffectiveAt,
		Major:         s.Bump == legal.BumpMajor,
		PolicyTitle:   up.Title.Pick(tmpl),
		Version:       up.Version.String(),
	})
	if err != nil {
		return nil, "", err
	}
	return &mail.Envelope{To: mail.ToUser(uid, u.Email(), u.Name()), Message: msg}, "", nil
}

// upcoming reads the catalog and returns the upcoming version of ref's type
// when it is ref.
func (r *policyNoticeResolver) upcoming(ctx context.Context, ref legal.Ref) (legal.Catalog, *legal.VersionRecord, error) {
	heads, err := r.repos.Versions.FindHeads(ctx)
	if err != nil {
		return legal.Catalog{}, nil, err
	}
	cat := legal.NewCatalog(heads)
	up := cat.Upcoming(ref.Type, r.now())
	if up == nil || up.Ref != ref {
		return cat, nil, nil
	}
	return cat, up, nil
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

func unlessPassEnded(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return nil
	}
	return err
}
