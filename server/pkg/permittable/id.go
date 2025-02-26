package permittable

import (
	"github.com/eukarya-inc/reearth-accounts/pkg/id"
)

type ID = id.PermittableID

var NewID = id.NewPermittableID

var MustID = id.MustPermittableID

var IDFrom = id.PermittableIDFrom

var IDFromRef = id.PermittableIDFromRef

var ErrInvalidID = id.ErrInvalidID
