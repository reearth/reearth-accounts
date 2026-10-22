// Package membership holds domain rules that span the Workspace and
// Permittable aggregates.
package membership

import (
	"errors"

	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
)

var (
	ErrNilWorkspace            = errors.New("membership: workspace is nil")
	ErrNilPermittable          = errors.New("membership: permittable is nil")
	ErrNilRole                 = errors.New("membership: role is nil")
	ErrPermittableUserMismatch = errors.New("membership: permittable belongs to a different user")
)

// ChangeMemberRole sets uid's role in ws to r and records the same role for ws
// on p, uid's Permittable.
//
// Invariant: for every user member of a workspace, workspace.Member.Role and
// the role recorded for that workspace on the user's Permittable agree. Every
// change to a member's role must go through this function, and the caller must
// save both ws and p in the same transaction; saving only one of them breaks
// the invariant.
//
// Either both aggregates are changed or neither is: on error, ws and p are left
// as they were.
//
// Errors: role.ErrInvalidRole when r's name is not a role.RoleType;
// ErrPermittableUserMismatch when p is not uid's;
// workspace.ErrCannotModifyPersonalWorkspace for a personal workspace;
// workspace.ErrTargetUserNotInTheWorkspace when uid is not a user member of ws.
func ChangeMemberRole(ws *workspace.Workspace, p *permittable.Permittable, uid user.ID, r *role.Role) error {
	if ws == nil {
		return ErrNilWorkspace
	}
	if p == nil {
		return ErrNilPermittable
	}
	if r == nil {
		return ErrNilRole
	}

	rt := role.RoleType(r.Name())
	if !rt.Valid() {
		return role.ErrInvalidRole
	}
	if p.UserID() != uid {
		return ErrPermittableUserMismatch
	}

	// Members.UpdateUserRole rejects a personal workspace and a non-member
	// without changing anything, so p is only touched once ws has changed.
	if err := ws.Members().UpdateUserRole(uid, rt); err != nil {
		return err
	}
	p.UpdateWorkspaceRole(ws.ID(), r.ID())
	return nil
}
