package adminuseruc

import (
	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/rerror"
)

// ErrCannotModifySelf is returned when an admin tries to approve/reject their
// own account.
var ErrCannotModifySelf = rerror.NewE(i18n.T("cannot modify your own admin account"))
