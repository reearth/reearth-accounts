package adminuser

import (
	"github.com/reearth/reearth-accounts/server/pkg/id"
)

type ID = id.AdminUserID
type IDList = id.AdminUserIDList

var NewID = id.NewAdminUserID

var MustID = id.MustAdminUserID

var IDFrom = id.AdminUserIDFrom

var IDFromRef = id.AdminUserIDFromRef

var ErrInvalidID = id.ErrInvalidID
