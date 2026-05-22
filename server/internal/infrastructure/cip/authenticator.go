package cip

import (
	"context"

	firebase "firebase.google.com/go/v4"
	fbauth "firebase.google.com/go/v4/auth"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/log"
	"github.com/reearth/reearthx/rerror"
)

// firebaseAuthClient is the narrow surface of the Firebase Admin SDK auth client
// used by the CIP Authenticator. Both *auth.Client and *auth.TenantClient satisfy
// it (they embed *baseClient), and it is faked in tests.
type firebaseAuthClient interface {
	UpdateUser(ctx context.Context, uid string, user *fbauth.UserToUpdate) (*fbauth.UserRecord, error)
	GetUser(ctx context.Context, uid string) (*fbauth.UserRecord, error)
	EmailVerificationLink(ctx context.Context, email string) (string, error)
}

// Params configures the CIP Authenticator (Firebase Admin SDK).
type Params struct {
	ProjectID string
	TenantID  string // optional GCIP multi-tenant scope
}

// Authenticator implements gateway.Authenticator backed by Cloud Identity Platform
// via the Firebase Admin SDK. It authenticates using Application Default Credentials
// (the Cloud Run service account).
type Authenticator struct {
	client firebaseAuthClient
}

var _ gateway.Authenticator = (*Authenticator)(nil)

// New constructs a CIP Authenticator using Application Default Credentials.
// When p.TenantID is set, management calls are scoped to that GCIP tenant.
func New(ctx context.Context, p Params) (*Authenticator, error) {
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: p.ProjectID})
	if err != nil {
		log.Errorf("cip: init firebase app: %+v", err)
		return nil, rerror.NewE(i18n.T("cip is not set up"))
	}
	authClient, err := app.Auth(ctx)
	if err != nil {
		log.Errorf("cip: init auth client: %+v", err)
		return nil, rerror.NewE(i18n.T("cip is not set up"))
	}

	var client firebaseAuthClient = authClient
	if p.TenantID != "" {
		tc, terr := authClient.TenantManager.AuthForTenant(p.TenantID)
		if terr != nil {
			log.Errorf("cip: init tenant client: %+v", terr)
			return nil, rerror.NewE(i18n.T("cip is not set up"))
		}
		client = tc
	}

	return &Authenticator{client: client}, nil
}

func (a *Authenticator) UpdateUser(ctx context.Context, p gateway.AuthenticatorUpdateUserParam) (gateway.AuthenticatorUser, error) {
	update := &fbauth.UserToUpdate{}
	changed := false
	if p.Name != nil {
		update = update.DisplayName(*p.Name)
		changed = true
	}
	if p.Email != nil {
		update = update.Email(*p.Email)
		changed = true
	}
	if p.Password != nil {
		update = update.Password(*p.Password)
		changed = true
	}
	if !changed {
		return gateway.AuthenticatorUser{}, rerror.NewE(i18n.T("nothing is updated"))
	}

	rec, err := a.client.UpdateUser(ctx, p.ID, update)
	if err != nil {
		log.Errorf("cip: update user: %+v", err)
		return gateway.AuthenticatorUser{}, rerror.NewE(i18n.T("failed to update user"))
	}
	return toAuthenticatorUser(rec), nil
}

func (a *Authenticator) ResendVerificationEmail(ctx context.Context, userID string) error {
	rec, err := a.client.GetUser(ctx, userID)
	if err != nil {
		log.Errorf("cip: get user for verification: %+v", err)
		return rerror.NewE(i18n.T("failed to resend verification email"))
	}
	if rec.UserInfo == nil || rec.Email == "" {
		return rerror.NewE(i18n.T("failed to resend verification email"))
	}

	// Generate the out-of-band verification link. Delivery is handled out-of-band
	// (Firebase email templates or the configured mailer); generating the link
	// validates the user and triggers Firebase-managed email when templates are on.
	if _, err := a.client.EmailVerificationLink(ctx, rec.Email); err != nil {
		log.Errorf("cip: email verification link: %+v", err)
		return rerror.NewE(i18n.T("failed to resend verification email"))
	}
	return nil
}

func toAuthenticatorUser(rec *fbauth.UserRecord) gateway.AuthenticatorUser {
	out := gateway.AuthenticatorUser{}
	if rec == nil {
		return out
	}
	out.EmailVerified = rec.EmailVerified
	if rec.UserInfo != nil {
		out.ID = rec.UID
		out.Name = rec.DisplayName
		out.Email = rec.Email
	}
	return out
}
