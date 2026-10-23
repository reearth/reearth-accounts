package workspaceuc

import (
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/rerror"
)

var (
	// ErrPersonalWorkspace is returned when ownership of a personal workspace
	// would change; a personal workspace always belongs to its single user.
	ErrPersonalWorkspace = rerror.NewE(i18n.T("personal workspace ownership cannot be transferred"))
	// ErrTargetNotMember is returned when the new owner is not a user member of
	// the workspace.
	ErrTargetNotMember = rerror.NewE(i18n.T("target user is not a member of the workspace"))
	// ErrTargetDisabled is returned when the new owner's membership is disabled.
	ErrTargetDisabled = rerror.NewE(i18n.T("target member is disabled"))
	// ErrTargetUserNotFound is returned when the new owner has a member entry but
	// no user record. It must not wrap rerror.ErrNotFound: that maps to 404,
	// which callers read as "workspace not found".
	ErrTargetUserNotFound = rerror.NewE(i18n.T("target user not found"))
	// ErrAlreadyOwner is returned, wrapped in an *AlreadyOwnerError, when the new
	// owner is already the workspace's only owner, so the transfer would change
	// nothing.
	ErrAlreadyOwner = rerror.NewE(i18n.T("target user is already the sole owner"))
	// ErrRoleNotConfigured is returned when the owner or maintainer role record
	// is missing. It must not wrap rerror.ErrNotFound: that maps to 404, which
	// callers read as "workspace not found".
	ErrRoleNotConfigured = rerror.NewE(i18n.T("workspace role is not configured"))
)

// AlreadyOwnerError is ErrAlreadyOwner together with the state that makes it
// true, so a caller retrying a transfer that already committed can still read
// the owner it asked for.
type AlreadyOwnerError struct {
	Workspace *workspace.Workspace
	NewOwner  *user.User
}

func (e *AlreadyOwnerError) Error() string { return ErrAlreadyOwner.Error() }

func (e *AlreadyOwnerError) Unwrap() error { return ErrAlreadyOwner }
