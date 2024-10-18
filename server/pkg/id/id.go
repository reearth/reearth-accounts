package id

import "github.com/reearth/reearthx/idx"

type Role struct{}
type Permittable struct{}

func (Role) Type() string        { return "role" }
func (Permittable) Type() string { return "permittable" }

type RoleID = idx.ID[Role]
type PermittableID = idx.ID[Permittable]

var NewRoleID = idx.New[Role]
var NewPermittableID = idx.New[Permittable]

var MustRoleID = idx.Must[Role]
var MustPermittableID = idx.Must[Permittable]

var RoleIDFrom = idx.From[Role]
var PermittableIDFrom = idx.From[Permittable]

var RoleIDFromRef = idx.FromRef[Role]
var PermittableIDFromRef = idx.FromRef[Permittable]

type RoleIDList = idx.List[Role]
type PermittableIDList = idx.List[Permittable]

var RoleIDListFrom = idx.ListFrom[Role]
var PermittableIDListFrom = idx.ListFrom[Permittable]

type RoleIDSet = idx.Set[Role]
type PermittableIDSet = idx.Set[Permittable]

var NewRoleIDSet = idx.NewSet[Role]
var NewPermittableIDSet = idx.NewSet[Permittable]
