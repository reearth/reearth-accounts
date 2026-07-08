package pgdoc

import (
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
	"github.com/reearth/reearth-accounts/server/pkg/id"
)

type AdminUserRow struct {
	ID         string
	Email      string
	Name       string
	PictureURL string
	Role       string
	Status     string
	ApprovedBy string
	ApprovedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func NewAdminUserRow(u adminuser.AdminUser) AdminUserRow {
	var approvedAt *time.Time
	if t := u.ApprovedAt(); !t.IsZero() {
		approvedAt = &t
	}
	approvedBy := ""
	if by := u.ApprovedBy(); !by.IsEmpty() {
		approvedBy = by.String()
	}
	return AdminUserRow{
		ID:         u.ID().String(),
		Email:      u.Email(),
		Name:       u.Name(),
		PictureURL: u.PictureURL(),
		Role:       u.Role().String(),
		Status:     u.Status().String(),
		ApprovedBy: approvedBy,
		ApprovedAt: approvedAt,
		CreatedAt:  u.CreatedAt(),
		UpdatedAt:  u.UpdatedAt(),
	}
}

func (r AdminUserRow) Model() (*adminuser.AdminUser, error) {
	uid, err := id.AdminUserIDFrom(r.ID)
	if err != nil {
		return nil, err
	}

	status, err := adminuser.StatusFrom(r.Status)
	if err != nil {
		return nil, err
	}

	b := adminuser.New().
		ID(uid).
		Email(r.Email).
		Name(r.Name).
		PictureURL(r.PictureURL).
		Status(status).
		UpdatedAt(r.UpdatedAt)

	// Tolerate empty/absent role so pre-migration rows still load; only error on
	// a present-but-invalid value.
	if r.Role != "" {
		role, err := adminuser.RoleFrom(r.Role)
		if err != nil {
			return nil, err
		}
		b = b.Role(role)
	}

	if r.ApprovedAt != nil {
		b = b.ApprovedAt(*r.ApprovedAt)
	}
	if r.ApprovedBy != "" {
		by, err := id.AdminUserIDFrom(r.ApprovedBy)
		if err != nil {
			return nil, err
		}
		b = b.ApprovedBy(by)
	}

	return b.Build()
}
