package gateway

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"golang.org/x/text/language"
)

type AuthenticatorUpdateUserParam struct {
	Email         *string
	EmailVerified *bool
	ID            string
	Name          *string
	Password      *string
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

// VerificationTarget names the user whose IdP account gets a new
// verification mail. UserID is the accounts user, Sub the IdP subject, and
// Lang the user's language for a mail accounts renders itself.
type VerificationTarget struct {
	Lang   language.Tag
	Sub    string
	UserID id.UserID
}

type Authenticator interface {
	DisableMFA(ctx context.Context, sub string) error
	EnableMFA(ctx context.Context, sub string) (enrollmentURL string, err error)
	GetMFAStatus(ctx context.Context, sub string) (MFAStatus, error)
	GetUser(ctx context.Context, userID string) (AuthenticatorUser, error)
	RegenerateMFARecoveryCode(ctx context.Context, sub string) (recoveryCode string, err error)
	ResendVerificationEmail(ctx context.Context, t VerificationTarget) error
	UpdateUser(context.Context, AuthenticatorUpdateUserParam) (AuthenticatorUser, error)
}
