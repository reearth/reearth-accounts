package authz

import (
	"context"
	"testing"

	"github.com/cerbos/cerbos-sdk-go/cerbos"
	adminrbac "github.com/reearth/reearth-accounts/server/internal/admin/rbac"
	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
	"github.com/stretchr/testify/assert"
)

// newTestClient returns a non-nil Cerbos client. cerbos.New is lazy and does
// not dial, so tests can drive the pre-Cerbos code paths (e.g. role validation)
// without a running Cerbos server.
func newTestClient(t *testing.T) *cerbos.GRPCClient {
	t.Helper()
	client, err := cerbos.New("localhost:3593", cerbos.WithPlaintext())
	if err != nil {
		t.Fatal(err)
	}
	return client
}

// A nil checker (a wiring bug) must fail closed with ErrNilChecker rather than
// panicking on the client dereference.
func TestChecker_Allowed_NilCheckerFailsClosed(t *testing.T) {
	var c *Checker

	allowed, err := c.Allowed(context.Background(), adminuser.NewID(), adminuser.RoleSystemAdmin, adminrbac.ResourceUser, adminrbac.ActionList)
	assert.ErrorIs(t, err, ErrNilChecker)
	assert.False(t, allowed)
}

// A nil client (Cerbos unconfigured, e.g. local dev) must bypass authorization.
func TestChecker_Allowed_NilClientBypasses(t *testing.T) {
	c := NewChecker(nil)

	allowed, err := c.Allowed(context.Background(), adminuser.NewID(), adminuser.RoleSystemAdmin, adminrbac.ResourceUser, adminrbac.ActionList)
	assert.NoError(t, err)
	assert.True(t, allowed)
}

// An unset or invalid role must be denied before Cerbos is ever dialed.
func TestChecker_Allowed_InvalidRoleDenies(t *testing.T) {
	c := NewChecker(newTestClient(t))

	// Empty role (unset) denies.
	allowed, err := c.Allowed(context.Background(), adminuser.NewID(), adminuser.Role(""), adminrbac.ResourceUser, adminrbac.ActionList)
	assert.NoError(t, err)
	assert.False(t, allowed)

	// Unknown role value denies.
	allowed, err = c.Allowed(context.Background(), adminuser.NewID(), adminuser.Role("bogus"), adminrbac.ResourceUser, adminrbac.ActionList)
	assert.NoError(t, err)
	assert.False(t, allowed)
}
