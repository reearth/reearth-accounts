package id

import "github.com/reearth/reearthx/idx"

type User struct{}
type Workspace struct{}
type Integration struct{}
type Role struct{}
type Permittable struct{}

func (User) Type() string        { return "user" }
func (Workspace) Type() string   { return "workspace" }
func (Integration) Type() string { return "integration" }
func (Role) Type() string        { return "role" }
func (Permittable) Type() string { return "permittable" }

type UserID = idx.ID[User]
type WorkspaceID = idx.ID[Workspace]
type IntegrationID = idx.ID[Integration]
type RoleID = idx.ID[Role]
type PermittableID = idx.ID[Permittable]

var NewUserID = idx.New[User]
var NewWorkspaceID = idx.New[Workspace]
var NewIntegrationID = idx.New[Integration]
var NewRoleID = idx.New[Role]
var NewPermittableID = idx.New[Permittable]

var MustUserID = idx.Must[User]
var MustWorkspaceID = idx.Must[Workspace]
var MustIntegrationID = idx.Must[Integration]
var MustRoleID = idx.Must[Role]
var MustPermittableID = idx.Must[Permittable]

var UserIDFrom = idx.From[User]
var WorkspaceIDFrom = idx.From[Workspace]
var IntegrationIDFrom = idx.From[Integration]
var RoleIDFrom = idx.From[Role]
var PermittableIDFrom = idx.From[Permittable]

var UserIDFromRef = idx.FromRef[User]
var WorkspaceIDFromRef = idx.FromRef[Workspace]
var IntegrationIDFromRef = idx.FromRef[Integration]
var RoleIDFromRef = idx.FromRef[Role]
var PermittableIDFromRef = idx.FromRef[Permittable]

type UserIDList = idx.List[User]
type WorkspaceIDList = idx.List[Workspace]
type IntegrationIDList = idx.List[Integration]
type RoleIDList = idx.List[Role]
type PermittableIDList = idx.List[Permittable]

var RoleIDListFrom = idx.ListFrom[Role]
var PermittableIDListFrom = idx.ListFrom[Permittable]

type RoleIDSet = idx.Set[Role]
type PermittableIDSet = idx.Set[Permittable]

var NewRoleIDSet = idx.NewSet[Role]
var NewPermittableIDSet = idx.NewSet[Permittable]
