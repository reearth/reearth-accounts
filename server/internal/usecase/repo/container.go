package repo

import (
	"github.com/reearth/reearthx/usecasex"
)

type Container struct {
	Role        Role
	Permittable Permittable
	Transaction usecasex.Transaction
}
