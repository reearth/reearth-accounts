package workspaceuc

import (
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
	// ErrAlreadyOwner is returned when the new owner is already the workspace's
	// only owner, so the transfer would change nothing.
	ErrAlreadyOwner = rerror.NewE(i18n.T("target user is already the sole owner"))
)
