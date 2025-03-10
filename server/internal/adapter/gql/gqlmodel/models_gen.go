// Code generated by github.com/99designs/gqlgen, DO NOT EDIT.

package gqlmodel

import (
	"fmt"
	"io"
	"strconv"
)

type Node interface {
	IsNode()
	GetID() ID
}

type WorkspaceMember interface {
	IsWorkspaceMember()
}

type AddIntegrationToWorkspaceInput struct {
	WorkspaceID   ID   `json:"workspaceId"`
	IntegrationID ID   `json:"integrationId"`
	Role          Role `json:"role"`
}

type AddRoleInput struct {
	Name string `json:"name"`
}

type AddRolePayload struct {
	Role *RoleForAuthorization `json:"role"`
}

type AddUsersToWorkspaceInput struct {
	WorkspaceID ID             `json:"workspaceId"`
	Users       []*MemberInput `json:"users"`
}

type AddUsersToWorkspacePayload struct {
	Workspace *Workspace `json:"workspace"`
}

type CheckPermissionInput struct {
	UserID   string `json:"userId"`
	Service  string `json:"service"`
	Resource string `json:"resource"`
	Action   string `json:"action"`
}

type CheckPermissionPayload struct {
	Allowed bool `json:"allowed"`
}

type CreateWorkspaceInput struct {
	Name string `json:"name"`
}

type CreateWorkspacePayload struct {
	Workspace *Workspace `json:"workspace"`
}

type DeleteMeInput struct {
	UserID ID `json:"userId"`
}

type DeleteMePayload struct {
	UserID ID `json:"userId"`
}

type DeleteWorkspaceInput struct {
	WorkspaceID ID `json:"workspaceId"`
}

type DeleteWorkspacePayload struct {
	WorkspaceID ID `json:"workspaceId"`
}

type GetUsersWithRolesPayload struct {
	UsersWithRoles []*UserWithRoles `json:"usersWithRoles"`
}

type Me struct {
	ID            ID           `json:"id"`
	Name          string       `json:"name"`
	Email         string       `json:"email"`
	Lang          string       `json:"lang"`
	Theme         Theme        `json:"theme"`
	MyWorkspaceID ID           `json:"myWorkspaceId"`
	Auths         []string     `json:"auths"`
	Workspaces    []*Workspace `json:"workspaces"`
	MyWorkspace   *Workspace   `json:"myWorkspace"`
}

type MemberInput struct {
	UserID ID   `json:"userId"`
	Role   Role `json:"role"`
}

type Mutation struct {
}

type Permittable struct {
	ID      ID   `json:"id"`
	UserID  ID   `json:"userId"`
	RoleIds []ID `json:"roleIds"`
}

type Query struct {
}

type RemoveIntegrationFromWorkspaceInput struct {
	WorkspaceID   ID `json:"workspaceId"`
	IntegrationID ID `json:"integrationId"`
}

type RemoveMemberFromWorkspacePayload struct {
	Workspace *Workspace `json:"workspace"`
}

type RemoveMyAuthInput struct {
	Auth string `json:"auth"`
}

type RemoveRoleInput struct {
	ID ID `json:"id"`
}

type RemoveRolePayload struct {
	ID ID `json:"id"`
}

type RemoveUserFromWorkspaceInput struct {
	WorkspaceID ID `json:"workspaceId"`
	UserID      ID `json:"userId"`
}

type RoleForAuthorization struct {
	ID   ID     `json:"id"`
	Name string `json:"name"`
}

type RolesPayload struct {
	Roles []*RoleForAuthorization `json:"roles"`
}

type UpdateIntegrationOfWorkspaceInput struct {
	WorkspaceID   ID   `json:"workspaceId"`
	IntegrationID ID   `json:"integrationId"`
	Role          Role `json:"role"`
}

type UpdateMeInput struct {
	Name                 *string `json:"name,omitempty"`
	Email                *string `json:"email,omitempty"`
	Lang                 *string `json:"lang,omitempty"`
	Theme                *Theme  `json:"theme,omitempty"`
	Password             *string `json:"password,omitempty"`
	PasswordConfirmation *string `json:"passwordConfirmation,omitempty"`
}

type UpdateMePayload struct {
	Me *Me `json:"me"`
}

type UpdateMemberOfWorkspacePayload struct {
	Workspace *Workspace `json:"workspace"`
}

type UpdatePermittableInput struct {
	UserID  ID   `json:"userId"`
	RoleIds []ID `json:"roleIds"`
}

type UpdatePermittablePayload struct {
	Permittable *Permittable `json:"permittable"`
}

type UpdateRoleInput struct {
	ID   ID     `json:"id"`
	Name string `json:"name"`
}

type UpdateRolePayload struct {
	Role *RoleForAuthorization `json:"role"`
}

type UpdateUserOfWorkspaceInput struct {
	WorkspaceID ID   `json:"workspaceId"`
	UserID      ID   `json:"userId"`
	Role        Role `json:"role"`
}

type UpdateWorkspaceInput struct {
	WorkspaceID ID     `json:"workspaceId"`
	Name        string `json:"name"`
}

type UpdateWorkspacePayload struct {
	Workspace *Workspace `json:"workspace"`
}

type User struct {
	ID    ID     `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (User) IsNode()        {}
func (this User) GetID() ID { return this.ID }

type UserWithRoles struct {
	User  *User                   `json:"user"`
	Roles []*RoleForAuthorization `json:"roles"`
}

type Workspace struct {
	ID       ID                `json:"id"`
	Name     string            `json:"name"`
	Members  []WorkspaceMember `json:"members"`
	Personal bool              `json:"personal"`
}

func (Workspace) IsNode()        {}
func (this Workspace) GetID() ID { return this.ID }

type WorkspaceIntegrationMember struct {
	IntegrationID ID    `json:"integrationId"`
	Role          Role  `json:"role"`
	Active        bool  `json:"active"`
	InvitedByID   ID    `json:"invitedById"`
	InvitedBy     *User `json:"invitedBy,omitempty"`
}

func (WorkspaceIntegrationMember) IsWorkspaceMember() {}

type WorkspaceUserMember struct {
	UserID ID    `json:"userId"`
	Role   Role  `json:"role"`
	User   *User `json:"user,omitempty"`
}

func (WorkspaceUserMember) IsWorkspaceMember() {}

type NodeType string

const (
	NodeTypeUser      NodeType = "USER"
	NodeTypeWorkspace NodeType = "WORKSPACE"
)

var AllNodeType = []NodeType{
	NodeTypeUser,
	NodeTypeWorkspace,
}

func (e NodeType) IsValid() bool {
	switch e {
	case NodeTypeUser, NodeTypeWorkspace:
		return true
	}
	return false
}

func (e NodeType) String() string {
	return string(e)
}

func (e *NodeType) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = NodeType(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid NodeType", str)
	}
	return nil
}

func (e NodeType) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type Role string

const (
	RoleReader     Role = "READER"
	RoleWriter     Role = "WRITER"
	RoleOwner      Role = "OWNER"
	RoleMaintainer Role = "MAINTAINER"
)

var AllRole = []Role{
	RoleReader,
	RoleWriter,
	RoleOwner,
	RoleMaintainer,
}

func (e Role) IsValid() bool {
	switch e {
	case RoleReader, RoleWriter, RoleOwner, RoleMaintainer:
		return true
	}
	return false
}

func (e Role) String() string {
	return string(e)
}

func (e *Role) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = Role(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid Role", str)
	}
	return nil
}

func (e Role) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type Theme string

const (
	ThemeDefault Theme = "DEFAULT"
	ThemeLight   Theme = "LIGHT"
	ThemeDark    Theme = "DARK"
)

var AllTheme = []Theme{
	ThemeDefault,
	ThemeLight,
	ThemeDark,
}

func (e Theme) IsValid() bool {
	switch e {
	case ThemeDefault, ThemeLight, ThemeDark:
		return true
	}
	return false
}

func (e Theme) String() string {
	return string(e)
}

func (e *Theme) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = Theme(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid Theme", str)
	}
	return nil
}

func (e Theme) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}
