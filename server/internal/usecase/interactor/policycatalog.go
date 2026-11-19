package interactor

import (
	"context"
	"sync"
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/legal"
)

// PolicyCatalogTTL bounds how long an instance serves policy versions that
// another instance's webhook may have changed.
const PolicyCatalogTTL = time.Minute

// PolicyCatalog caches the policy catalog per process.
type PolicyCatalog struct {
	loadedAt time.Time
	mu       sync.Mutex
	now      func() time.Time
	repo     legal.VersionRepo
	snapshot *legal.Catalog
	ttl      time.Duration
}

func NewPolicyCatalog(r legal.VersionRepo, ttl time.Duration) *PolicyCatalog {
	return &PolicyCatalog{now: time.Now, repo: r, ttl: ttl}
}

// Get returns the cached catalog, reloading it when it is older than the TTL.
func (c *PolicyCatalog) Get(ctx context.Context) (legal.Catalog, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.snapshot != nil && c.now().Sub(c.loadedAt) < c.ttl {
		return *c.snapshot, nil
	}
	return c.load(ctx)
}

func (c *PolicyCatalog) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.snapshot = nil
}

// Refresh reloads the catalog whatever its age.
func (c *PolicyCatalog) Refresh(ctx context.Context) (legal.Catalog, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.load(ctx)
}

func (c *PolicyCatalog) load(ctx context.Context) (legal.Catalog, error) {
	at := c.now()
	heads, err := c.repo.FindHeads(ctx)
	if err != nil {
		return legal.Catalog{}, err
	}
	records, err := c.repo.FindVersions(ctx)
	if err != nil {
		return legal.Catalog{}, err
	}
	cat := legal.NewCatalog(heads, records)
	c.snapshot, c.loadedAt = &cat, at
	return cat, nil
}
