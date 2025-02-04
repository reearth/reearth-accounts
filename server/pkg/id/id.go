package id

import "github.com/reearth/reearthx/idx"

type Role struct{}
type Group struct{}
type Permittable struct{}

func (Role) Type() string        { return "role" }
func (Group) Type() string        { return "group" }
func (Permittable) Type() string { return "permittable" }

type RoleID = idx.ID[Role]
type GroupID = idx.ID[Group]
type PermittableID = idx.ID[Permittable]

var NewRoleID = idx.New[Role]
var NewGroupID = idx.New[Group]
var NewPermittableID = idx.New[Permittable]

var MustRoleID = idx.Must[Role]
var MustGroupID = idx.Must[Group]
var MustPermittableID = idx.Must[Permittable]

var RoleIDFrom = idx.From[Role]
var GroupIDFrom = idx.From[Group]
var PermittableIDFrom = idx.From[Permittable]

var RoleIDFromRef = idx.FromRef[Role]
var GroupIDFromRef = idx.FromRef[Group]
var PermittableIDFromRef = idx.FromRef[Permittable]

type RoleIDList = idx.List[Role]
type GroupIDList = idx.List[Group]
type PermittableIDList = idx.List[Permittable]

var RoleIDListFrom = idx.ListFrom[Role]
var GroupIDListFrom = idx.ListFrom[Group]
var PermittableIDListFrom = idx.ListFrom[Permittable]

type RoleIDSet = idx.Set[Role]
type GroupIDSet = idx.Set[Group]
type PermittableIDSet = idx.Set[Permittable]

var NewRoleIDSet = idx.NewSet[Role]
var NewGroupIDSet = idx.NewSet[Group]
var NewPermittableIDSet = idx.NewSet[Permittable]
