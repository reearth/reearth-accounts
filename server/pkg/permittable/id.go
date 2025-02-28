package permittable

import (
	"github.com/reearth/reearth-accounts/pkg/id"
)

type ID = id.PermittableID

var NewID = id.NewPermittableID

var MustID = id.MustPermittableID

var IDFrom = id.PermittableIDFrom

var IDFromRef = id.PermittableIDFromRef

var ErrInvalidID = id.ErrInvalidID
