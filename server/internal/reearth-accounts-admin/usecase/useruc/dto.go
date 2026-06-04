package useruc

import (
	"github.com/reearth/reearth-accounts/server/pkg/user"
)

// UserDTO represents a user for admin API responses.
type UserDTO struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Alias string `json:"alias"`
}

func toUserDTO(u *user.User) *UserDTO {
	if u == nil {
		return nil
	}
	return &UserDTO{
		ID:    u.ID().String(),
		Name:  u.Name(),
		Email: u.Email(),
		Alias: u.Alias(),
	}
}
