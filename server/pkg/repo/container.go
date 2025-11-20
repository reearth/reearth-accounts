package repo

import (
	"github.com/reearth/reearthx/usecasex"
)

type Container struct {
	Workspace   Workspace
	User        User
	Transaction usecasex.Transaction
}
