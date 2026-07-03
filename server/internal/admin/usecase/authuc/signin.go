// Package authuc holds the admin authentication usecases: exchanging a Google
// id_token for an admin session and loading the current admin user.
package authuc

import (
	"context"
	"errors"
	"strings"

	"github.com/reearth/reearth-accounts/server/internal/admin/gateway/google"
	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/rerror"
)

var (
	// ErrInvalidToken is returned when the Google id_token cannot be verified.
	ErrInvalidToken = rerror.NewE(i18n.T("invalid id token"))
	// ErrEmailNotVerified is returned when the Google account's email is not verified.
	ErrEmailNotVerified = rerror.NewE(i18n.T("email not verified"))
	// ErrDomainNotAllowed is returned when the account is not in the allowed domain.
	ErrDomainNotAllowed = rerror.NewE(i18n.T("email domain not allowed"))
)

// GoogleSignInOptions configures the sign-in policy.
type GoogleSignInOptions struct {
	AllowedDomain   string
	BootstrapEmails []string
}

// GoogleSignInUseCase verifies a Google id_token and upserts the corresponding
// admin user (pending, or approved when the email is bootstrapped).
type GoogleSignInUseCase struct {
	repo          adminuser.Repo
	verifier      google.Verifier
	allowedDomain string
	bootstrap     map[string]bool
}

// NewGoogleSignInUseCase is a Wire provider for GoogleSignInUseCase.
func NewGoogleSignInUseCase(repo adminuser.Repo, verifier google.Verifier, opts GoogleSignInOptions) *GoogleSignInUseCase {
	bootstrap := make(map[string]bool, len(opts.BootstrapEmails))
	for _, e := range opts.BootstrapEmails {
		if n := adminuser.NormalizeEmail(e); n != "" {
			bootstrap[n] = true
		}
	}
	return &GoogleSignInUseCase{
		repo:          repo,
		verifier:      verifier,
		allowedDomain: strings.ToLower(strings.TrimSpace(opts.AllowedDomain)),
		bootstrap:     bootstrap,
	}
}

// Execute verifies the id_token, enforces the domain/verification policy, and
// returns the (created or existing) admin user.
func (uc *GoogleSignInUseCase) Execute(ctx context.Context, idToken string) (*adminuser.AdminUser, error) {
	claims, err := uc.verifier.Verify(ctx, idToken)
	if err != nil || claims == nil {
		return nil, ErrInvalidToken
	}
	if !claims.EmailVerified {
		return nil, ErrEmailNotVerified
	}

	email := adminuser.NormalizeEmail(claims.Email)
	if email == "" {
		return nil, ErrInvalidToken
	}
	if err := uc.checkDomain(email, claims.HD); err != nil {
		return nil, err
	}

	u, err := uc.repo.FindByEmail(ctx, email)
	if err != nil && !errors.Is(err, rerror.ErrNotFound) {
		return nil, err
	}

	if u != nil {
		// Refresh the profile from the latest Google data on each sign-in.
		// Name and picture are updated independently, and an empty Google claim
		// keeps the stored value (UpdateProfile requires a non-empty name).
		name := u.Name()
		if claims.Name != "" {
			name = claims.Name
		}
		picture := u.PictureURL()
		if claims.PictureURL != "" {
			picture = claims.PictureURL
		}
		if name != u.Name() || picture != u.PictureURL() {
			if err := u.UpdateProfile(name, picture); err != nil {
				return nil, err
			}
			if err := uc.repo.Save(ctx, u); err != nil {
				return nil, err
			}
		}
		return u, nil
	}

	// new account: pending unless bootstrapped
	b := adminuser.New().NewID().Email(email).Name(displayName(claims.Name, email)).PictureURL(claims.PictureURL)
	if uc.bootstrap[email] {
		b = b.Status(adminuser.StatusApproved)
	} else {
		b = b.Status(adminuser.StatusPending)
	}
	created, err := b.Build()
	if err != nil {
		return nil, err
	}
	if err := uc.repo.Save(ctx, created); err != nil {
		return nil, err
	}
	return created, nil
}

func (uc *GoogleSignInUseCase) checkDomain(email, hd string) error {
	if uc.allowedDomain == "" {
		return nil
	}
	if !strings.EqualFold(hd, uc.allowedDomain) {
		return ErrDomainNotAllowed
	}
	if !strings.HasSuffix(email, "@"+uc.allowedDomain) {
		return ErrDomainNotAllowed
	}
	return nil
}

// displayName falls back to the local part of the email when Google supplies no
// name (the domain requires a non-empty name).
func displayName(name, email string) string {
	if name != "" {
		return name
	}
	if i := strings.IndexByte(email, '@'); i > 0 {
		return email[:i]
	}
	return email
}
