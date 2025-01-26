package group

import (
	"github.com/eukarya-inc/reearth-dashboard/pkg/id"
)

type ID = id.GroupID

var NewID = id.NewGroupID

var MustID = id.MustGroupID

var IDFrom = id.GroupIDFrom

var IDFromRef = id.GroupIDFromRef

var ErrInvalidID = id.ErrInvalidID
