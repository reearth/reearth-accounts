package httpmodel

import (
	"time"

	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/util"
)

// UserMetadataResponse mirrors UserMetadata.
type UserMetadataResponse struct {
	Description string `json:"description"`
	Website     string `json:"website"`
	PhotoURL    string `json:"photo_url"`
	Lang        string `json:"lang"`
	Theme       string `json:"theme"`
}

// VerificationResponse mirrors Verification.
type VerificationResponse struct {
	Code       string `json:"code"`
	Expiration string `json:"expiration"`
	Verified   bool   `json:"verified"`
}

// UserResponse mirrors the GraphQL User type.
type UserResponse struct {
	ID           string                `json:"id"`
	Name         string                `json:"name"`
	Alias        string                `json:"alias"`
	Email        string                `json:"email"`
	Host         *string               `json:"host,omitempty"`
	Workspace    string                `json:"workspace"`
	Auths        []string              `json:"auths"`
	Metadata     *UserMetadataResponse `json:"metadata"`
	Verification *VerificationResponse `json:"verification,omitempty"`
}

// MeResponse mirrors the GraphQL Me type.
type MeResponse struct {
	ID             string                `json:"id"`
	Name           string                `json:"name"`
	Alias          string                `json:"alias"`
	Email          string                `json:"email"`
	Metadata       *UserMetadataResponse `json:"metadata"`
	Host           *string               `json:"host,omitempty"`
	LatestLogoutAt *time.Time            `json:"latest_logout_at,omitempty"`
	MyWorkspaceID  string                `json:"my_workspace_id"`
	Auths          []string              `json:"auths"`
}

// SimpleUserResponse mirrors the user.Simple shape used by userByNameOrEmail.
type SimpleUserResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func metadataResponse(u *user.User) *UserMetadataResponse {
	m := u.Metadata()
	return &UserMetadataResponse{
		Description: m.Description(),
		Website:     m.Website(),
		PhotoURL:    m.PhotoURL(),
		Lang:        m.Lang().String(),
		Theme:       string(m.Theme()),
	}
}

// NewUserResponse converts a domain user to a UserResponse.
func NewUserResponse(u *user.User) *UserResponse {
	if u == nil {
		return nil
	}
	var v *VerificationResponse
	if u.Verification() != nil {
		v = &VerificationResponse{
			Code:       u.Verification().Code(),
			Expiration: u.Verification().Expiration().Format("2006-01-02T15:04:05.000Z"),
			Verified:   u.Verification().IsVerified(),
		}
	}
	host := u.Host()
	var hp *string
	if host != "" {
		hp = &host
	}
	return &UserResponse{
		ID:           u.ID().String(),
		Name:         u.Name(),
		Alias:        u.Alias(),
		Email:        u.Email(),
		Host:         hp,
		Workspace:    u.Workspace().String(),
		Auths:        util.Map(u.Auths(), func(a user.Auth) string { return a.Provider }),
		Metadata:     metadataResponse(u),
		Verification: v,
	}
}

// NewUserResponses converts a domain user list.
func NewUserResponses(ul user.List) []*UserResponse {
	out := make([]*UserResponse, 0, len(ul))
	for _, u := range ul {
		out = append(out, NewUserResponse(u))
	}
	return out
}

// NewMeResponse converts a domain user to a MeResponse.
func NewMeResponse(u *user.User) *MeResponse {
	if u == nil {
		return nil
	}
	var ll *time.Time
	if !u.LatestLogoutAt().IsZero() {
		t := u.LatestLogoutAt()
		ll = &t
	}
	host := u.Host()
	var hp *string
	if host != "" {
		hp = &host
	}
	return &MeResponse{
		ID:             u.ID().String(),
		Name:           u.Name(),
		Alias:          u.Alias(),
		Email:          u.Email(),
		Metadata:       metadataResponse(u),
		Host:           hp,
		LatestLogoutAt: ll,
		MyWorkspaceID:  u.Workspace().String(),
		Auths:          util.Map(u.Auths(), func(a user.Auth) string { return a.Provider }),
	}
}

// NewSimpleUserResponse converts a *user.Simple.
func NewSimpleUserResponse(u *user.Simple) *SimpleUserResponse {
	if u == nil {
		return nil
	}
	return &SimpleUserResponse{ID: u.ID.String(), Name: u.Name, Email: u.Email}
}

// --- Request DTOs ---

// UpdateMeRequest mirrors updateMe input.
type UpdateMeRequest struct {
	Alias                *string `json:"alias,omitempty"`
	Description          *string `json:"description,omitempty"`
	Email                *string `json:"email,omitempty" validate:"omitempty,email"`
	Lang                 *string `json:"lang,omitempty"`
	Name                 *string `json:"name,omitempty"`
	Password             *string `json:"password,omitempty"`
	PasswordConfirmation *string `json:"password_confirmation,omitempty"`
	PhotoURL             *string `json:"photo_url,omitempty"`
	Theme                *string `json:"theme,omitempty" validate:"omitempty,oneof=default dark light"`
	Website              *string `json:"website,omitempty"`
}

// ToInteractorInput converts to interfaces.UpdateMeParam.
func (r *UpdateMeRequest) ToInteractorInput() interfaces.UpdateMeParam {
	return interfaces.UpdateMeParam{
		Alias:                r.Alias,
		Description:          r.Description,
		Email:                r.Email,
		Lang:                 ParseLang(r.Lang),
		Name:                 r.Name,
		Password:             r.Password,
		PasswordConfirmation: r.PasswordConfirmation,
		PhotoURL:             r.PhotoURL,
		Theme:                ParseTheme(r.Theme),
		Website:              r.Website,
	}
}

// SignupRequest mirrors signup input.
type SignupRequest struct {
	ID          *string `json:"id,omitempty"`
	WorkspaceID *string `json:"workspace_id,omitempty"`
	Name        string  `json:"name" validate:"required"`
	Email       string  `json:"email" validate:"required,email"`
	Password    string  `json:"password" validate:"required"`
	Secret      *string `json:"secret,omitempty"`
	Lang        *string `json:"lang,omitempty"`
	Theme       *string `json:"theme,omitempty" validate:"omitempty,oneof=default dark light"`
	MockAuth    *bool   `json:"mock_auth,omitempty"`
}

// SignupOIDCRequest mirrors signupOIDC input.
type SignupOIDCRequest struct {
	ID          *string `json:"id,omitempty"`
	WorkspaceID *string `json:"workspace_id,omitempty"`
	Name        *string `json:"name,omitempty"`
	Email       *string `json:"email,omitempty" validate:"omitempty,email"`
	Sub         *string `json:"sub,omitempty"`
	Lang        *string `json:"lang,omitempty"`
	Secret      *string `json:"secret,omitempty"`
}

// CreateVerificationRequest mirrors createVerification input.
type CreateVerificationRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// VerifyUserRequest mirrors verifyUser input.
type VerifyUserRequest struct {
	Code string `json:"code" validate:"required"`
}

// StartPasswordResetRequest mirrors startPasswordReset input.
type StartPasswordResetRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// PasswordResetRequest mirrors passwordReset input.
type PasswordResetRequest struct {
	Password string `json:"password" validate:"required"`
	Token    string `json:"token" validate:"required"`
}

// FindOrCreateRequest mirrors findOrCreate input.
type FindOrCreateRequest struct {
	Sub   string `json:"sub" validate:"required"`
	Iss   string `json:"iss" validate:"required"`
	Token string `json:"token" validate:"required"`
}

// MessageResponse is a simple {success:true} body for void mutations.
type MessageResponse struct {
	Success bool `json:"success"`
}
