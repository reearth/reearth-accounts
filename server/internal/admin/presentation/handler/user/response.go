package user

import (
	"github.com/reearth/reearth-accounts/server/pkg/user"
)

// UserResponse is a single user in the admin API list.
type UserResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Alias string `json:"alias"`
} // @name User

// ListUsersResponse is the paginated list of users.
type ListUsersResponse struct {
	Items      []UserResponse `json:"items"`
	TotalCount int64          `json:"totalCount"`
	Page       int64          `json:"page"`
	PerPage    int64          `json:"perPage"`
} // @name ListUsersResponse

// UserDetailResponse is the detail view of a single user.
type UserDetailResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Alias string `json:"alias"`
	Host  string `json:"host"`
} // @name UserDetail

// UserWorkspaceResponse is a workspace a user belongs to, with the user's role.
type UserWorkspaceResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Alias    string `json:"alias"`
	Personal bool   `json:"personal"`
	Role     string `json:"role"`
} // @name UserWorkspace

func newUserDetailResponse(u *user.User) UserDetailResponse {
	return UserDetailResponse{
		ID:    u.ID().String(),
		Name:  u.Name(),
		Email: u.Email(),
		Alias: u.Alias(),
		Host:  u.Host(),
	}
}

func newUserResponse(u *user.User) UserResponse {
	return UserResponse{
		ID:    u.ID().String(),
		Name:  u.Name(),
		Email: u.Email(),
		Alias: u.Alias(),
	}
}

func newUserResponses(list user.List) []UserResponse {
	items := make([]UserResponse, 0, len(list))
	for _, u := range list {
		items = append(items, newUserResponse(u))
	}
	return items
}
