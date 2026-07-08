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
