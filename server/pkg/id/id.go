package id

import "github.com/reearth/reearthx/idx"

type Role struct{}
type Permittable struct{}
type User struct{}
type Workspace struct{}

func (Role) Type() string        { return "role" }
func (Permittable) Type() string { return "permittable" }
func (User) Type() string        { return "user" }
func (Workspace) Type() string   { return "workspace" }

type RoleID = idx.ID[Role]
type PermittableID = idx.ID[Permittable]
type UserID = idx.ID[User]
type WorkspaceID = idx.ID[Workspace]

var NewRoleID = idx.New[Role]
var NewPermittableID = idx.New[Permittable]
var NewUserID = idx.New[User]
var NewWorkspaceID = idx.New[Workspace]

var MustRoleID = idx.Must[Role]
var MustPermittableID = idx.Must[Permittable]
var MustUserID = idx.Must[User]
var MustWorkspaceID = idx.Must[Workspace]

var RoleIDFrom = idx.From[Role]
var PermittableIDFrom = idx.From[Permittable]
var UserIDFrom = idx.From[User]
var WorkspaceIDFrom = idx.From[Workspace]

var RoleIDFromRef = idx.FromRef[Role]
var PermittableIDFromRef = idx.FromRef[Permittable]
var UserIDFromRef = idx.FromRef[User]
var WorkspaceIDFromRef = idx.FromRef[Workspace]

type RoleIDList = idx.List[Role]
type PermittableIDList = idx.List[Permittable]
type UserIDList = idx.List[User]
type WorkspaceIDList = idx.List[Workspace]

var RoleIDListFrom = idx.ListFrom[Role]
var PermittableIDListFrom = idx.ListFrom[Permittable]
var UserIDListFrom = idx.ListFrom[User]
var WorkspaceIDListFrom = idx.ListFrom[Workspace]

type RoleIDSet = idx.Set[Role]
type PermittableIDSet = idx.Set[Permittable]
type UserIDSet = idx.Set[User]
type WorkspaceIDSet = idx.Set[Workspace]

var NewRoleIDSet = idx.NewSet[Role]
var NewPermittableIDSet = idx.NewSet[Permittable]
var NewUserIDSet = idx.NewSet[User]
var NewWorkspaceIDSet = idx.NewSet[Workspace]
