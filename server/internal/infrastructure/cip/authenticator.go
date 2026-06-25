package cip

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"time"

	firebase "firebase.google.com/go/v4"
	fbauth "firebase.google.com/go/v4/auth"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/log"
	"github.com/reearth/reearthx/mailer"
	"github.com/reearth/reearthx/rerror"
	"google.golang.org/api/option"
)

// firebaseAuthClient is the narrow surface of the Firebase Admin SDK auth client
// used by the CIP Authenticator. Both *auth.Client and *auth.TenantClient satisfy
// it (they embed *baseClient), and it is faked in tests.
type firebaseAuthClient interface {
	UpdateUser(ctx context.Context, uid string, user *fbauth.UserToUpdate) (*fbauth.UserRecord, error)
	GetUser(ctx context.Context, uid string) (*fbauth.UserRecord, error)
	EmailVerificationLink(ctx context.Context, email string) (string, error)
}

const defaultHTTPTimeout = 5 * time.Second

// Params configures the CIP Authenticator (Firebase Admin SDK).
type Params struct {
	HTTPTimeout time.Duration // defaults to 5s when zero
	ProjectID   string
	TenantID    string // optional GCIP multi-tenant scope
}

// Authenticator implements gateway.Authenticator backed by Cloud Identity Platform
// via the Firebase Admin SDK. It authenticates using Application Default Credentials
// (the Cloud Run service account).
type Authenticator struct {
	client firebaseAuthClient
	mailer mailer.Mailer
}

var _ gateway.Authenticator = (*Authenticator)(nil)

// New constructs a CIP Authenticator using Application Default Credentials.
// When p.TenantID is set, management calls are scoped to that GCIP tenant.
func New(ctx context.Context, p Params, m mailer.Mailer) (*Authenticator, error) {
	// Fail fast on an inconsistent configuration: selecting CIP without a project
	// id means Config.Auths() never registers the CIP JWT provider (so CIP tokens
	// would not validate) while management calls would still be routed to Firebase.
	if p.ProjectID == "" {
		return nil, rerror.NewE(i18n.T("cip project id is required"))
	}

	timeout := p.HTTPTimeout
	if timeout <= 0 {
		timeout = defaultHTTPTimeout
	}
	httpClient := &http.Client{Timeout: timeout}
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: p.ProjectID}, option.WithHTTPClient(httpClient))
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

	return &Authenticator{client: client, mailer: m}, nil
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

	// The Firebase Admin SDK's EmailVerificationLink only generates an
	// out-of-band verification URL; it does not send any email itself.
	// Delivery is handled below via the configured app Mailer.
	link, err := a.client.EmailVerificationLink(ctx, rec.Email)
	if err != nil {
		log.Errorf("cip: email verification link: %+v", err)
		return rerror.NewE(i18n.T("failed to resend verification email"))
	}
	if a.mailer == nil {
		log.Errorf("cip: mailer is not configured; cannot send verification email")
		return rerror.NewE(i18n.T("failed to resend verification email"))
	}

	text := "Please verify your email address by opening the following link:\n" + link
	// Treat the link as untrusted input and HTML-escape it before embedding in the
	// href attribute to prevent accidental injection or broken markup.
	htmlBody := fmt.Sprintf(`<p>Please verify your email address by clicking <a href="%s">this link</a>.</p>`, html.EscapeString(link))
	if err := a.mailer.SendMail(ctx, []mailer.Contact{{Email: rec.Email, Name: rec.DisplayName}}, "email verification", text, htmlBody); err != nil {
		log.Errorf("cip: send verification email: %+v", err)
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
