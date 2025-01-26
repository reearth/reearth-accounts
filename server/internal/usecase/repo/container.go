package repo

import (
	"github.com/reearth/reearthx/usecasex"
)

type Container struct {
	Role        Role
	Group       Group
	Permittable Permittable
	Transaction usecasex.Transaction
}
