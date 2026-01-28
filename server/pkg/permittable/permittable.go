package permittable

import (
	"time"

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
	updatedAt      time.Time
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
	p.updatedAt = time.Now()
}

func (p *Permittable) EditWorkspaceRoles(workspaceRoles []WorkspaceRole) {
	if p == nil {
		return
	}

	p.workspaceRoles = workspaceRoles
	p.updatedAt = time.Now()
}

func (p *Permittable) RemoveWorkspaceRole(wId workspace.ID) {
	if p == nil {
		return
	}

	for i, wr := range p.workspaceRoles {
		if wr.id == wId {
			p.workspaceRoles = append(p.workspaceRoles[:i], p.workspaceRoles[i+1:]...)
			p.updatedAt = time.Now()
			return
		}
	}
}

func (p *Permittable) UpdatedAt() time.Time {
	if p == nil {
		return time.Time{}
	}
	return p.updatedAt
}

func (p *Permittable) UpdateWorkspaceRole(wId workspace.ID, rId role.ID) {
	if p == nil {
		return
	}

	for i, wr := range p.workspaceRoles {
		if wr.id == wId {
			p.workspaceRoles[i].roleID = rId
			p.updatedAt = time.Now()
			return
		}
	}

	p.workspaceRoles = append(p.workspaceRoles, NewWorkspaceRole(wId, rId))
	p.updatedAt = time.Now()
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
