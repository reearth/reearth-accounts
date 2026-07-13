package adminuser

import (
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
)

// AdminUserResponse is a single admin user in the admin API.
type AdminUserResponse struct {
	ID         string     `json:"id"`
	Email      string     `json:"email"`
	Name       string     `json:"name"`
	PictureURL string     `json:"pictureUrl"`
	Role       string     `json:"role"`
	Status     string     `json:"status"`
	ApprovedBy string     `json:"approvedBy,omitempty"`
	ApprovedAt *time.Time `json:"approvedAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
} // @name AdminUser

// ListAdminUsersResponse is the paginated list of admin users.
type ListAdminUsersResponse struct {
	Items      []AdminUserResponse `json:"items"`
	TotalCount int64               `json:"totalCount"`
	Page       int64               `json:"page"`
	PerPage    int64               `json:"perPage"`
} // @name ListAdminUsersResponse

func newAdminUserResponse(u *adminuser.AdminUser) AdminUserResponse {
	res := AdminUserResponse{
		ID:         u.ID().String(),
		Email:      u.Email(),
		Name:       u.Name(),
		PictureURL: u.PictureURL(),
		Role:       u.Role().String(),
		Status:     u.Status().String(),
		CreatedAt:  u.CreatedAt(),
		UpdatedAt:  u.UpdatedAt(),
	}
	if by := u.ApprovedBy(); !by.IsEmpty() {
		res.ApprovedBy = by.String()
	}
	if at := u.ApprovedAt(); !at.IsZero() {
		res.ApprovedAt = &at
	}
	return res
}

func newAdminUserResponses(list adminuser.List) []AdminUserResponse {
	items := make([]AdminUserResponse, 0, len(list))
	for _, u := range list {
		items = append(items, newAdminUserResponse(u))
	}
	return items
}
