package interactor

import (
	"context"
	"io"
	"time"

	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/legal"
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

// ReceiveWebhook checks the secret, then the signature over the exact bytes
// received, then every field, before it writes anything.
func (p *Policy) ReceiveWebhook(ctx context.Context, r io.Reader, signature string) (legal.ApplyResult, error) {
	if !p.secret.Enabled() {
		return legal.ApplyResult{}, legal.ErrNotConfigured
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
