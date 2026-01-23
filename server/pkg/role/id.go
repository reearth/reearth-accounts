package role

import (
	"github.com/reearth/reearth-accounts/server/pkg/id"
)

type ID = id.RoleID
type IDList = id.RoleIDList

var NewID = id.NewRoleID

var MustID = id.MustRoleID

var IDFrom = id.RoleIDFrom

var IDFromRef = id.RoleIDFromRef

var ErrInvalidID = id.ErrInvalidID
