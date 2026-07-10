// Package authz evaluates admin authorization against the "accounts-admin"
// Cerbos service. It talks to Cerbos directly (independent of the GraphQL
// presentation layer) so the admin binary stays isolated.
package authz

import (
	"context"
	"fmt"

	"github.com/cerbos/cerbos-sdk-go/cerbos"
	effectv1 "github.com/cerbos/cerbos/api/genpb/cerbos/effect/v1"
	adminrbac "github.com/reearth/reearth-accounts/server/internal/admin/rbac"
	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
)

// Checker builds a Cerbos principal from the admin's role and evaluates it
// against the admin policy set. The role lives directly on the AdminUser
// aggregate, so no repository lookup is required.
type Checker struct {
	client *cerbos.GRPCClient
}

// NewChecker is a Wire provider for the admin authorization checker. A nil
// client (Cerbos unconfigured) makes every check pass, for local development.
func NewChecker(client *cerbos.GRPCClient) *Checker {
	return &Checker{client: client}
}

// Allowed reports whether an admin with the given role may perform action on
// resource within the admin Cerbos service.
func (c *Checker) Allowed(ctx context.Context, principalID adminuser.ID, role adminuser.Role, resource, action string) (bool, error) {
	if c.client == nil {
		return true, nil
	}

	// An unset or invalid role denies before any Cerbos call is made.
	if !role.Valid() {
		return false, nil
	}

	principal := cerbos.NewPrincipal(principalID.String(), role.String())
	kind := fmt.Sprintf("%s:%s", adminrbac.ServiceName, resource)
	resourceID := fmt.Sprintf("%s:%s:%s", adminrbac.ServiceName, resource, principalID.String())

	batch := cerbos.NewResourceBatch()
	batch.Add(cerbos.NewResource(kind, resourceID), action)

	resp, err := c.client.CheckResources(ctx, principal, batch)
	if err != nil {
		return false, err
	}
	if resp == nil {
		return false, nil
	}
	for _, result := range resp.Results {
		if result.Actions[action] == effectv1.Effect_EFFECT_ALLOW {
			return true, nil
		}
	}
	return false, nil
}
