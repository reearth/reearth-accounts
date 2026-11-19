package interactor

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/legal"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/rerror"
)

type Policy struct {
	catalog *PolicyCatalog
	now     func() time.Time
	repos   *repo.Container
	secret  legal.Secret
}

func NewPolicy(r *repo.Container, secret legal.Secret, catalog *PolicyCatalog) *Policy {
	return &Policy{catalog: catalog, now: time.Now, repos: r, secret: secret}
}

func (p *Policy) Accept(ctx context.Context, in []legal.RefInput, client legal.Client, op *workspace.Operator) (*user.User, error) {
	if !p.secret.Enabled() {
		return nil, legal.ErrNotConfigured
	}
	if op == nil || op.User == nil {
		return nil, interfaces.ErrInvalidOperator
	}
	refs, err := legal.ParseRefs(in)
	if err != nil {
		return nil, err
	}
	uid := *op.User
	held, err := p.repos.PolicyAcceptance.FindByUser(ctx, uid)
	if err != nil {
		return nil, err
	}
	now := p.now().UTC().Truncate(time.Millisecond)
	plan, err := p.planWithFreshCatalog(ctx, held, refs, now)
	if err != nil {
		return nil, err
	}
	records := make([]legal.Acceptance, len(plan))
	for i, r := range plan {
		records[i] = legal.Acceptance{AcceptedAt: now, Client: client, Ref: r, UserID: uid}
	}
	if err := p.repos.PolicyAcceptance.InsertIfAbsent(ctx, records); err != nil {
		return nil, err
	}
	u, err := p.repos.User.FindByID(ctx, uid)
	if errors.Is(err, rerror.ErrNotFound) {
		// DeleteMe may have removed the user after this request authenticated,
		// and its sweep may have run before the insert above.
		if rmErr := p.repos.PolicyAcceptance.RemoveByUser(ctx, uid); rmErr != nil {
			return nil, rmErr
		}
	}
	return u, err
}

func (p *Policy) Overview(ctx context.Context, op *workspace.Operator) (legal.Overview, error) {
	if op == nil || op.User == nil {
		return legal.Overview{}, interfaces.ErrInvalidOperator
	}
	if !p.secret.Enabled() {
		return legal.EmptyOverview(), nil
	}
	held, err := p.repos.PolicyAcceptance.FindByUser(ctx, *op.User)
	if err != nil {
		return legal.Overview{}, err
	}
	cat, err := p.catalog.Get(ctx)
	if err != nil {
		return legal.Overview{}, err
	}
	return legal.BuildOverview(cat, held, p.now()), nil
}

func (p *Policy) ReceiveWebhook(ctx context.Context, r io.Reader, signature string) (legal.ApplyResult, error) {
	if !p.secret.Enabled() {
		return legal.ApplyResult{}, legal.ErrNotConfigured
	}
	if _, err := legal.ParseSignature(signature); err != nil {
		return legal.ApplyResult{}, err
	}
	body, err := legal.ReadWebhookBody(r)
	if err != nil {
		return legal.ApplyResult{}, err
	}
	if err := p.secret.Verify(body, signature); err != nil {
		return legal.ApplyResult{}, err
	}
	m, err := legal.ParseManifest(body, p.now())
	if err != nil {
		return legal.ApplyResult{}, err
	}
	res, err := legal.Apply(ctx, p.repos.PolicyVersion, m, p.now)
	p.catalog.Invalidate()
	return res, err
}

// planWithFreshCatalog reloads the catalog once before it rejects a version,
// because the webhook may have reached another instance within the cache TTL.
func (p *Policy) planWithFreshCatalog(ctx context.Context, held []legal.Acceptance, refs []legal.Ref, now time.Time) ([]legal.Ref, error) {
	cat, err := p.catalog.Get(ctx)
	if err != nil {
		return nil, err
	}
	plan, err := legal.PlanAccept(cat, held, refs, now)
	if !errors.Is(err, legal.ErrUnknownType) && !errors.Is(err, legal.ErrNotAcceptable) {
		return plan, err
	}
	if cat, err = p.catalog.Refresh(ctx); err != nil {
		return nil, err
	}
	return legal.PlanAccept(cat, held, refs, now)
}
