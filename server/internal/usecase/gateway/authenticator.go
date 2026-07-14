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

type MFAStatus struct {
	Enrolled bool
}

type Authenticator interface {
	DisableMFA(ctx context.Context, sub string) error
	EnableMFA(ctx context.Context, sub string) (enrollmentURL string, err error)
	GetMFAStatus(ctx context.Context, sub string) (MFAStatus, error)
	ResendVerificationEmail(ctx context.Context, userID string) error
	UpdateUser(context.Context, AuthenticatorUpdateUserParam) (AuthenticatorUser, error)
}
