package pgdoc

import (
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
)

// PermittableWorkspaceRoleRow mirrors a permittable_workspace_roles row.
type PermittableWorkspaceRoleRow struct {
	PermittableID string
	WorkspaceID   string
	RoleID        string
}

type PermittableRow struct {
	ID        string
	UserID    string
	RoleIDs   []string
	UpdatedAt time.Time
}

// NewPermittableRow decomposes a permittable into its parent row plus the
// workspace-role child rows (mirroring mongodoc, which persists workspace_roles).
func NewPermittableRow(p permittable.Permittable) (PermittableRow, []PermittableWorkspaceRoleRow) {
	roleIDs := make([]string, 0, len(p.RoleIDs()))
	for _, rid := range p.RoleIDs() {
		roleIDs = append(roleIDs, rid.String())
	}

	updatedAt := p.UpdatedAt()
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}

	pid := p.ID().String()
	wrs := make([]PermittableWorkspaceRoleRow, 0, len(p.WorkspaceRoles()))
	for _, wr := range p.WorkspaceRoles() {
		wrs = append(wrs, PermittableWorkspaceRoleRow{
			PermittableID: pid,
			WorkspaceID:   wr.ID().String(),
			RoleID:        wr.RoleID().String(),
		})
	}

	return PermittableRow{
		ID:        pid,
		UserID:    p.UserID().String(),
		RoleIDs:   roleIDs,
		UpdatedAt: updatedAt,
	}, wrs
}

func PermittableModel(r PermittableRow, wrs []PermittableWorkspaceRoleRow) (*permittable.Permittable, error) {
	pid, err := id.PermittableIDFrom(r.ID)
	if err != nil {
		return nil, err
	}
	uid, err := id.UserIDFrom(r.UserID)
	if err != nil {
		return nil, err
	}
	roleIDs := make(id.RoleIDList, 0, len(r.RoleIDs))
	for _, s := range r.RoleIDs {
		rid, err := id.RoleIDFrom(s)
		if err != nil {
			return nil, err
		}
		roleIDs = append(roleIDs, rid)
	}

	var workspaceRoles []permittable.WorkspaceRole
	if len(wrs) > 0 {
		workspaceRoles = make([]permittable.WorkspaceRole, 0, len(wrs))
		for _, wr := range wrs {
			wid, err := id.WorkspaceIDFrom(wr.WorkspaceID)
			if err != nil {
				return nil, err
			}
			rid, err := id.RoleIDFrom(wr.RoleID)
			if err != nil {
				return nil, err
			}
			workspaceRoles = append(workspaceRoles, permittable.NewWorkspaceRole(wid, rid))
		}
	}

	return permittable.New().
		ID(pid).
		UserID(uid).
		RoleIDs(roleIDs).
		WorkspaceRoles(workspaceRoles).
		UpdatedAt(r.UpdatedAt).
		Build()
}
