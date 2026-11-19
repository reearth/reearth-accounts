// Package policyuc records policy notices and reports their mail. The admin
// API never sends mail; the accounts server's passes queue and send it.
package policyuc

import (
	"context"
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/legal"
	"github.com/reearth/reearth-accounts/server/pkg/outbox"
)

type CreateNoticeInput struct {
	Audience    legal.Audience
	RequestedBy id.AdminUserID
	Type        legal.Type
}

type CreateNoticeUseCase struct {
	notices  legal.NoticeRepo
	now      func() time.Time
	versions legal.VersionRepo
}

func NewCreateNoticeUseCase(versions legal.VersionRepo, notices legal.NoticeRepo) *CreateNoticeUseCase {
	return &CreateNoticeUseCase{notices: notices, now: func() time.Time { return time.Now().UTC() }, versions: versions}
}

// Execute records a notice for the upcoming version of in.Type, or returns
// the stored one for the same version and audience with created false.
func (uc *CreateNoticeUseCase) Execute(ctx context.Context, in CreateNoticeInput) (legal.Notice, bool, error) {
	cat, err := catalog(ctx, uc.versions)
	if err != nil {
		return legal.Notice{}, false, err
	}
	n, err := legal.NewNotice(cat, in.Type, in.Audience, in.RequestedBy, uc.now())
	if err != nil {
		return legal.Notice{}, false, err
	}
	return uc.notices.Create(ctx, n)
}

type NoticeStatus struct {
	Counts map[outbox.State]int
	Notice legal.Notice
}

type ListNoticesUseCase struct {
	notices  legal.NoticeRepo
	now      func() time.Time
	outbox   outbox.Repo
	versions legal.VersionRepo
}

func NewListNoticesUseCase(versions legal.VersionRepo, notices legal.NoticeRepo, ob outbox.Repo) *ListNoticesUseCase {
	return &ListNoticesUseCase{notices: notices, now: func() time.Time { return time.Now().UTC() }, outbox: ob, versions: versions}
}

// Execute returns the notices of t's upcoming version with their record
// counts per outbox state. It returns none when t has no upcoming version.
func (uc *ListNoticesUseCase) Execute(ctx context.Context, t legal.Type) ([]NoticeStatus, error) {
	cat, err := catalog(ctx, uc.versions)
	if err != nil {
		return nil, err
	}
	up := cat.Upcoming(t, uc.now())
	if up == nil {
		return []NoticeStatus{}, nil
	}
	ns, err := uc.notices.FindByRef(ctx, up.Ref)
	if err != nil {
		return nil, err
	}
	res := make([]NoticeStatus, 0, len(ns))
	for _, n := range ns {
		counts, err := uc.outbox.CountByNotice(ctx, n.ID)
		if err != nil {
			return nil, err
		}
		res = append(res, NoticeStatus{Counts: counts, Notice: n})
	}
	return res, nil
}

func catalog(ctx context.Context, versions legal.VersionRepo) (legal.Catalog, error) {
	heads, err := versions.FindHeads(ctx)
	if err != nil {
		return legal.Catalog{}, err
	}
	return legal.NewCatalog(heads), nil
}
