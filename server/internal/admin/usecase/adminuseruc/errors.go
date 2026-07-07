package adminuseruc

import (
	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/rerror"
)

var (
	// ErrCannotModifySelf is returned when an admin tries to approve/reject
	// their own account.
	ErrCannotModifySelf = rerror.NewE(i18n.T("cannot modify your own admin account"))
	// ErrLastApprovedAdmin is returned when rejecting would remove the last
	// approved admin.
	ErrLastApprovedAdmin = rerror.NewE(i18n.T("cannot reject the last approved admin"))
)
