package mongodoc

import (
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	permittable "github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/user"
)

type WorkspaceRoleDocument struct {
	WorkspaceID string `json:"workspace_id" bson:"workspace_id" jsonschema:"foreignkey=workspace,description=Workspace ID (ULID format)"`
	RoleID      string `json:"role_id" bson:"role_id" jsonschema:"foreignkey=role,description=Role ID (ULID format)"`
}

type PermittableDocument struct {
	ID             string                  `json:"id" bson:"id" jsonschema:"required,description=Permittable ID (ULID format)"`
	UserID         string                  `json:"userid" bson:"userid" jsonschema:"required,foreignkey=user,description=User ID this permittable represents"`
	RoleIDs        []string                `json:"roleids" bson:"roleids" jsonschema:"foreignkey=role,description=List of role IDs assigned to this user. Default: []"`
	WorkspaceRoles []WorkspaceRoleDocument `json:"workspace_roles,omitempty" bson:"workspace_roles,omitempty" jsonschema:"description=Workspace-specific role assignments"`
	UpdatedAt      time.Time               `json:"updatedat" bson:"updatedat" jsonschema:"description=Last update timestamp"`
}

type PermittableConsumer = Consumer[*PermittableDocument, *permittable.Permittable]

func NewPermittableConsumer() *PermittableConsumer {
	return NewConsumer[*PermittableDocument, *permittable.Permittable](func(a *permittable.Permittable) bool {
		return true
	})
}

func NewPermittable(p permittable.Permittable) (*PermittableDocument, string) {
	id := p.ID().String()

	roleIds := make([]string, 0, len(p.RoleIDs()))
	for _, r := range p.RoleIDs() {
		roleIds = append(roleIds, r.String())
	}

	var workspaceRoles []WorkspaceRoleDocument
	if len(p.WorkspaceRoles()) > 0 {
		workspaceRoles = make([]WorkspaceRoleDocument, 0, len(p.WorkspaceRoles()))
		for _, r := range p.WorkspaceRoles() {
			workspaceRoles = append(workspaceRoles, WorkspaceRoleDocument{
				WorkspaceID: r.ID().String(),
				RoleID:      r.RoleID().String(),
			})
		}
	}

	updatedAt := p.UpdatedAt()
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}

	return &PermittableDocument{
		ID:             id,
		UserID:         p.UserID().String(),
		RoleIDs:        roleIds,
		WorkspaceRoles: workspaceRoles,
		UpdatedAt:      updatedAt,
	}, id
}

func (d *PermittableDocument) Model() (*permittable.Permittable, error) {
	if d == nil {
		return nil, nil
	}

	uid, err := id.PermittableIDFrom(d.ID)
	if err != nil {
		return nil, err
	}

	userId, err := user.IDFrom(d.UserID)
	if err != nil {
		return nil, err
	}

	roleIds, err := id.RoleIDListFrom(d.RoleIDs)
	if err != nil {
		return nil, err
	}

	var workspaceRoles []permittable.WorkspaceRole
	if len(d.WorkspaceRoles) > 0 {
		workspaceRoles = make([]permittable.WorkspaceRole, 0, len(d.WorkspaceRoles))
		for _, r := range d.WorkspaceRoles {
			workspaceID, wErr := id.WorkspaceIDFrom(r.WorkspaceID)
			if wErr != nil {
				return nil, wErr
			}

			roleID, rErr := id.RoleIDFrom(r.RoleID)
			if rErr != nil {
				return nil, rErr
			}

			workspaceRole := permittable.NewWorkspaceRole(workspaceID, roleID)
			workspaceRoles = append(workspaceRoles, workspaceRole)
		}
	}

	return permittable.New().
		ID(uid).
		UpdatedAt(d.UpdatedAt).
		UserID(userId).
		RoleIDs(roleIds).
		WorkspaceRoles(workspaceRoles).
		Build()
}
