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

// Accept reloads the catalog once before it rejects a version, because the
// webhook may have reached another instance within the cache TTL.
func (p *Policy) Accept(ctx context.Context, refs []legal.Ref, client legal.Client, op *workspace.Operator) (*user.User, error) {
	if !p.secret.Enabled() {
		return nil, legal.ErrNotConfigured
	}
	if op == nil || op.User == nil {
		return nil, interfaces.ErrInvalidOperator
	}
	uid := *op.User
	held, err := p.repos.PolicyAcceptance.FindByUser(ctx, uid)
	if err != nil {
		return nil, err
	}
	cat, err := p.catalog.Get(ctx)
	if err != nil {
		return nil, err
	}
	now := p.now()
	plan, err := legal.PlanAccept(cat, held, refs, now)
	if errors.Is(err, legal.ErrUnknownType) || errors.Is(err, legal.ErrNotAcceptable) {
		if cat, err = p.catalog.Refresh(ctx); err != nil {
			return nil, err
		}
		plan, err = legal.PlanAccept(cat, held, refs, now)
	}
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
	return p.repos.User.FindByID(ctx, uid)
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

// ReceiveWebhook checks the secret, then the signature over the exact bytes
// received, then every field, before it writes anything. A request without
// a well-formed signature is refused before its body is read.
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
	if len(res.Applied) > 0 {
		p.catalog.Invalidate()
	}
	return res, err
}
