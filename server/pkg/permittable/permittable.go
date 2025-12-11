package permittable

import (
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
)

type Permittable struct {
	id             ID
	userID         user.ID
	roleIDs        []role.ID
	workspaceRoles []WorkspaceRole
}

type WorkspaceRole struct {
	id     workspace.ID
	roleID role.ID
}

func NewWorkspaceRole(workspaceID workspace.ID, roleID role.ID) WorkspaceRole {
	return WorkspaceRole{
		id:     workspaceID,
		roleID: roleID,
	}
}

func (p *Permittable) ID() ID {
	if p == nil {
		return ID{}
	}
	return p.id
}

func (p *Permittable) UserID() user.ID {
	if p == nil {
		return user.ID{}
	}
	return p.userID
}

func (p *Permittable) RoleIDs() []id.RoleID {
	if p == nil {
		return nil
	}
	return p.roleIDs
}

func (p *Permittable) WorkspaceRoles() []WorkspaceRole {
	if p == nil {
		return nil
	}

	return p.workspaceRoles
}

func (p *Permittable) EditRoleIDs(roleIDs id.RoleIDList) {
	if p == nil {
		return
	}
	p.roleIDs = roleIDs
}

func (p *Permittable) EditWorkspaceRoles(workspaceRoles []WorkspaceRole) {
	if p == nil {
		return
	}

	p.workspaceRoles = workspaceRoles
}

func (p *WorkspaceRole) ID() workspace.ID {
	if p == nil {
		return workspace.ID{}
	}

	return p.id
}

func (p *WorkspaceRole) RoleID() role.ID {
	if p == nil {
		return role.ID{}
	}

	return p.roleID
}
