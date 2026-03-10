package gateway

import "context"

type AuthenticatorUpdateUserParam struct {
	ID       string
	Name     *string
	Email    *string
	Password *string
}

type AuthenticatorUser struct {
	ID            string
	Name          string
	Email         string
	EmailVerified bool
}

type Authenticator interface {
	ResendVerificationEmail(ctx context.Context, userID string) error
	UpdateUser(context.Context, AuthenticatorUpdateUserParam) (AuthenticatorUser, error)
	ValidatePassword(ctx context.Context, email, password string) (bool, error)
}
