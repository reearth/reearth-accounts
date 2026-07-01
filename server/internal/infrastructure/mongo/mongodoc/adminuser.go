package mongodoc

import (
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
	"github.com/reearth/reearth-accounts/server/pkg/id"
)

type AdminUserDocument struct {
	ID         string    `json:"id" bson:"id" jsonschema:"required,description=Admin user ID (ULID format)"`
	Email      string    `json:"email" bson:"email" jsonschema:"required,description=Admin user email address (lowercase, unique)"`
	Name       string    `json:"name" bson:"name" jsonschema:"required,description=Admin user display name"`
	PictureURL string    `json:"pictureurl" bson:"pictureurl" jsonschema:"description=Admin user picture URL from Google profile. Default: \"\""`
	Status     string    `json:"status" bson:"status" jsonschema:"required,description=Admin user status: pending, approved or rejected"`
	ApprovedBy string    `json:"approvedby" bson:"approvedby" jsonschema:"description=ID of the admin who approved this user (ULID format). Default: \"\""`
	ApprovedAt time.Time `json:"approvedat" bson:"approvedat" jsonschema:"description=Approval timestamp"`
	CreatedAt  time.Time `json:"createdat" bson:"createdat" jsonschema:"description=Creation timestamp"`
	UpdatedAt  time.Time `json:"updatedat" bson:"updatedat" jsonschema:"description=Last update timestamp"`
}

type AdminUserConsumer = Consumer[*AdminUserDocument, *adminuser.AdminUser]

func NewAdminUserConsumer() *AdminUserConsumer {
	return NewConsumer[*AdminUserDocument, *adminuser.AdminUser](func(a *adminuser.AdminUser) bool {
		return true
	})
}

func NewAdminUser(u adminuser.AdminUser) (*AdminUserDocument, string) {
	uid := u.ID().String()

	updatedAt := u.UpdatedAt()
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}

	approvedBy := ""
	if by := u.ApprovedBy(); !by.IsEmpty() {
		approvedBy = by.String()
	}

	return &AdminUserDocument{
		ID:         uid,
		Email:      u.Email(),
		Name:       u.Name(),
		PictureURL: u.PictureURL(),
		Status:     u.Status().String(),
		ApprovedBy: approvedBy,
		ApprovedAt: u.ApprovedAt(),
		CreatedAt:  u.CreatedAt(),
		UpdatedAt:  updatedAt,
	}, uid
}

func (d *AdminUserDocument) Model() (*adminuser.AdminUser, error) {
	if d == nil {
		return nil, nil
	}

	uid, err := id.AdminUserIDFrom(d.ID)
	if err != nil {
		return nil, err
	}

	status, err := adminuser.StatusFrom(d.Status)
	if err != nil {
		return nil, err
	}

	b := adminuser.New().
		ID(uid).
		Email(d.Email).
		Name(d.Name).
		PictureURL(d.PictureURL).
		Status(status).
		ApprovedAt(d.ApprovedAt).
		UpdatedAt(d.UpdatedAt)

	if d.ApprovedBy != "" {
		by, err := id.AdminUserIDFrom(d.ApprovedBy)
		if err != nil {
			return nil, err
		}
		b = b.ApprovedBy(by)
	}

	return b.Build()
}
