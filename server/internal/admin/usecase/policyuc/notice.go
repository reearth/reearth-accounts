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

type CreateNoticeResult struct {
	Notice  legal.Notice
	Outcome legal.CreateOutcome
}

func (uc *CreateNoticeUseCase) Execute(ctx context.Context, in CreateNoticeInput) (CreateNoticeResult, error) {
	cat, err := catalog(ctx, uc.versions)
	if err != nil {
		return CreateNoticeResult{}, err
	}
	n, err := legal.NewNotice(cat, in.Type, in.Audience, in.RequestedBy, uc.now())
	if err != nil {
		return CreateNoticeResult{}, err
	}
	stored, outcome, err := uc.notices.Create(ctx, n)
	if err != nil {
		return CreateNoticeResult{}, err
	}
	return CreateNoticeResult{Notice: stored, Outcome: outcome}, nil
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
