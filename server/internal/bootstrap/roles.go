// Package bootstrap holds startup steps shared by the reearth-accounts and
// reearth-accounts-admin binaries.
package bootstrap

import (
	"context"
	"errors"
	"fmt"

	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearthx/log"
	"github.com/reearth/reearthx/rerror"
)

// SeedMockAuthRoles saves a role record for every role.RoleTypes() name that
// is missing. It does nothing unless mockAuth is true. Usecases look roles up
// by name and fail when a record is missing, so callers must run it before
// the server starts accepting requests.
func SeedMockAuthRoles(ctx context.Context, mockAuth bool, roles role.Repo) error {
	if !mockAuth {
		return nil
	}
	for _, rt := range role.RoleTypes() {
		_, err := roles.FindByName(ctx, rt.String())
		if err == nil {
			continue
		}
		if !errors.Is(err, rerror.ErrNotFound) {
			return fmt.Errorf("find role %q: %w", rt, err)
		}
		r, err := role.New().NewID().Name(rt.String()).Build()
		if err != nil {
			return fmt.Errorf("build role %q: %w", rt, err)
		}
		if err := roles.Save(ctx, *r); err != nil {
			return fmt.Errorf("save role %q: %w", rt, err)
		}
		log.Infofc(ctx, "[MockAuth] seeded role: %s", rt)
	}
	return nil
}
