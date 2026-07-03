package auth

import (
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
)

// GoogleSignInRequest is the body of POST /auth/google.
type GoogleSignInRequest struct {
	IDToken string `json:"id_token" validate:"required"`
} // @name GoogleSignInRequest

// GoogleSignInResponse is returned after a successful Google sign-in. It carries
// just enough for the frontend to route to the pending/approved/rejected screen.
type GoogleSignInResponse struct {
	Status     string `json:"status"`
	Email      string `json:"email"`
	Name       string `json:"name"`
	PictureURL string `json:"pictureUrl"`
} // @name GoogleSignInResponse

// MeResponse is the current admin user's record returned by GET /me.
type MeResponse struct {
	ID         string     `json:"id"`
	Email      string     `json:"email"`
	Name       string     `json:"name"`
	PictureURL string     `json:"pictureUrl"`
	Status     string     `json:"status"`
	ApprovedBy string     `json:"approvedBy,omitempty"`
	ApprovedAt *time.Time `json:"approvedAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
} // @name MeResponse

func newGoogleSignInResponse(u *adminuser.AdminUser) GoogleSignInResponse {
	return GoogleSignInResponse{
		Status:     u.Status().String(),
		Email:      u.Email(),
		Name:       u.Name(),
		PictureURL: u.PictureURL(),
	}
}

func newMeResponse(u *adminuser.AdminUser) MeResponse {
	res := MeResponse{
		ID:         u.ID().String(),
		Email:      u.Email(),
		Name:       u.Name(),
		PictureURL: u.PictureURL(),
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
