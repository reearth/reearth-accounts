package interactor

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/samber/lo"
	"golang.org/x/text/language"

	accountmemory "github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/mailer"
	"github.com/reearth/reearthx/rerror"
	"github.com/reearth/reearthx/util"

	"github.com/stretchr/testify/assert"
)

func TestUser_Signup(t *testing.T) {
	user.DefaultPasswordEncoder = &user.NoopPasswordEncoder{}
	uid := id.NewUserID()
	tid := id.NewWorkspaceID()
	mocktime := time.Time{}
	mockcode := "CODECODE"

	tests := []struct {
		name             string
		signupSecret     string
		authSrvUIDomain  string
		createUserBefore *user.User
		args             interfaces.SignupParam
		wantUser         func(u *user.User) *user.User
		wantWorkspace    *workspace.Workspace
		wantMailTo       []mailer.Contact
		wantMailSubject  string
		wantMailContent  string
		wantError        error
	}{
		{
			name: "duplicate user alias",
			signupSecret: "",
			authSrvUIDomain: "",
			createUserBefore: user.New().
				ID(uid).
				Workspace(tid).
				Name("NAME").
				Alias("user-" + uid.String()).
				Email("aaa@bbb.com").
				MustBuild(),
			args: interfaces.SignupParam{
				Email:       "other@bbb.com",
				Name:        "NAME",
				Password:    "PAss00!!",
				UserID:      &uid,
				WorkspaceID: &tid,
			},
			wantUser:      nil,
			wantWorkspace: nil,
			wantError:     interfaces.ErrUserAliasAlreadyExists,
		},
	{
		name: "duplicate workspace alias",
		signupSecret: "",
		authSrvUIDomain: "",
		createUserBefore: nil,
		args: interfaces.SignupParam{
			Email:       "unique@bbb.com",
			Name:        "NAME",
			Password:    "PAss00!!",
			UserID:      &uid,
			WorkspaceID: &tid,
		},
		wantUser:      nil,
		wantWorkspace: nil,
		wantError:     interfaces.ErrWorkspaceAliasAlreadyExists,
	},
		{
			name:            "without secret",
			signupSecret:    "",
			authSrvUIDomain: "https://reearth.io",
			args: interfaces.SignupParam{
				Email:       "aaa@bbb.com",
				Name:        "NAME",
				Password:    "PAss00!!",
				UserID:      &uid,
				WorkspaceID: &tid,
			},
			wantUser: func(u *user.User) *user.User {
				return user.New().
					ID(uid).
					Workspace(tid).
					Name("NAME").
					Alias("user-" + uid.String()).
					Auths(u.Auths()).
					Metadata(*u.Metadata()).
					Email("aaa@bbb.com").
					PasswordPlainText("PAss00!!").
					Verification(user.VerificationFrom(mockcode, mocktime.Add(24*time.Hour), false)).
					MustBuild()
			},
			wantWorkspace: workspace.New().
				ID(tid).
				Name("NAME").
				Alias("user-" + uid.String()).
				Members(map[user.ID]workspace.Member{uid: {Role: workspace.RoleOwner, Disabled: false, InvitedBy: uid}}).
				Personal(true).
				MustBuild(),
			wantMailTo:      []mailer.Contact{{Email: "aaa@bbb.com", Name: "NAME"}},
			wantMailSubject: "email verification",
			wantMailContent: "https://reearth.io/?user-verification-token=CODECODE",
			wantError:       nil,
		},
		{
			name:            "existing but not valdiated user",
			signupSecret:    "",
			authSrvUIDomain: "",
			createUserBefore: user.New().
				ID(uid).
				Workspace(tid).
				Name("NAME").
				Email("aaa@bbb.com").
				MustBuild(),
			args: interfaces.SignupParam{
				Email:       "aaa@bbb.com",
				Name:        "NAME",
				Password:    "PAss00!!",
				UserID:      &uid,
				WorkspaceID: &tid,
			},
			wantUser:      nil,
			wantWorkspace: nil,
			wantError:     interfaces.ErrUserAlreadyExists,
		},
		{
			name:            "existing and valdiated user",
			signupSecret:    "",
			authSrvUIDomain: "",
			createUserBefore: user.New().
				ID(uid).
				Workspace(tid).
				Email("aaa@bbb.com").
				Name("NAME").
				Verification(user.VerificationFrom(mockcode, mocktime, true)).
				MustBuild(),
			args: interfaces.SignupParam{
				Email:       "aaa@bbb.com",
				Name:        "NAME",
				Password:    "PAss00!!",
				UserID:      &uid,
				WorkspaceID: &tid,
			},
			wantUser:      nil,
			wantWorkspace: nil,
			wantError:     interfaces.ErrUserAlreadyExists,
		},
		{
			name:            "without secret 2",
			signupSecret:    "",
			authSrvUIDomain: "",
			args: interfaces.SignupParam{
				Email:       "aaa@bbb.com",
				Name:        "NAME",
				Password:    "PAss00!!",
				Secret:      lo.ToPtr("hogehoge"),
				UserID:      &uid,
				WorkspaceID: &tid,
			},
			wantUser: func(u *user.User) *user.User {
				return user.New().
					ID(uid).
					Workspace(tid).
					Name("NAME").
					Alias("user-" + uid.String()).
					Auths(u.Auths()).
					Metadata(*u.Metadata()).
					Email("aaa@bbb.com").
					PasswordPlainText("PAss00!!").
					Verification(user.VerificationFrom(mockcode, mocktime.Add(24*time.Hour), false)).
					MustBuild()
			},
			wantWorkspace: workspace.New().
				ID(tid).
				Name("NAME").
				Alias("user-" + uid.String()).
				Members(map[user.ID]workspace.Member{uid: {Role: workspace.RoleOwner, Disabled: false, InvitedBy: uid}}).
				Personal(true).
				MustBuild(),
			wantMailTo:      []mailer.Contact{{Email: "aaa@bbb.com", Name: "NAME"}},
			wantMailSubject: "email verification",
			wantMailContent: "/?user-verification-token=CODECODE",
			wantError:       nil,
		},
		{
			name:            "with secret",
			signupSecret:    "SECRET",
			authSrvUIDomain: "",
			args: interfaces.SignupParam{
				Email:       "aaa@bbb.com",
				Name:        "NAME",
				Password:    "PAss00!!",
				Secret:      lo.ToPtr("SECRET"),
				UserID:      &uid,
				WorkspaceID: &tid,
				Lang:        &language.Japanese,
				Theme:       user.ThemeDark.Ref(),
			},
			wantUser: func(u *user.User) *user.User {
				metadata := user.NewMetadata()
				metadata.LangFrom(language.Japanese.String())
				metadata.SetTheme(user.ThemeDark)

				return user.New().
					ID(uid).
					Workspace(tid).
					Name("NAME").
					Alias("user-" + uid.String()).
					Auths(u.Auths()).
					Email("aaa@bbb.com").
					PasswordPlainText("PAss00!!").
					Metadata(metadata).
					Verification(user.VerificationFrom(mockcode, mocktime.Add(24*time.Hour), false)).
					MustBuild()
			},
			wantWorkspace: workspace.New().
				ID(tid).
				Name("NAME").
				Alias("user-" + uid.String()).
				Members(map[user.ID]workspace.Member{uid: {Role: workspace.RoleOwner, Disabled: false, InvitedBy: uid}}).
				Personal(true).
				MustBuild(),
			wantMailTo:      []mailer.Contact{{Email: "aaa@bbb.com", Name: "NAME"}},
			wantMailSubject: "email verification",
			wantMailContent: "/?user-verification-token=CODECODE",
			wantError:       nil,
		},
		{
			name:            "invalid secret",
			signupSecret:    "SECRET",
			authSrvUIDomain: "",
			args: interfaces.SignupParam{
				Email:    "aaa@bbb.com",
				Name:     "NAME",
				Password: "PAss00!!",
				Secret:   lo.ToPtr("SECRET!"),
			},
			wantError: interfaces.ErrSignupInvalidSecret,
		},
		{
			name:            "invalid secret 2",
			signupSecret:    "SECRET",
			authSrvUIDomain: "",
			args: interfaces.SignupParam{
				Email:    "aaa@bbb.com",
				Name:     "NAME",
				Password: "PAss00!!",
			},
			wantError: interfaces.ErrSignupInvalidSecret,
		},
		{
			name: "invalid email",
			args: interfaces.SignupParam{
				Email:    "aaa",
				Name:     "NAME",
				Password: "PAss00!!",
			},
			wantError: user.ErrInvalidEmail,
		},
		{
			name: "invalid password",
			args: interfaces.SignupParam{
				Email:    "aaa@bbb.com",
				Name:     "NAME",
				Password: "PAss00",
			},
			wantError: user.ErrPasswordLength,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			// t.Parallel() cannot be used because Now and GenerateVerificationCode are mocked

			defer util.MockNow(mocktime)()
			defer user.MockGenerateVerificationCode(mockcode)()

			ctx := context.Background()
			r := accountmemory.New()
			if tt.createUserBefore != nil {
				assert.NoError(t, r.User.Save(ctx, tt.createUserBefore))
			}

			m := mailer.NewMock()
			g := &gateway.Container{Mailer: m}
			uc := NewUser(r, g, tt.signupSecret, tt.authSrvUIDomain)
			u, err := uc.Signup(ctx, tt.args)

			if tt.wantUser != nil {
				assert.Equal(t, tt.wantUser(u), u)
			} else {
				assert.Nil(t, u)
			}

			var ws *workspace.Workspace
			if u != nil {
				ws, _ = r.Workspace.FindByID(ctx, u.Workspace())
			}
			assert.Equal(t, tt.wantWorkspace, ws)

			assert.Equal(t, tt.wantError, err)

			mails := m.Mails()
			if tt.wantMailSubject == "" {
				assert.Empty(t, mails)
			} else {
				assert.Equal(t, 1, len(mails))
				assert.Equal(t, tt.wantMailSubject, mails[0].Subject)
				assert.Equal(t, tt.wantMailTo, mails[0].To)
				assert.Contains(t, mails[0].PlainContent, tt.wantMailContent)
			}
		})
	}
}

func TestIssToURL(t *testing.T) {
	assert.Nil(t, issToURL("", ""))
	assert.Equal(t, &url.URL{Scheme: "https", Host: "iss.com"}, issToURL("iss.com", ""))
	assert.Equal(t, &url.URL{Scheme: "https", Host: "iss.com"}, issToURL("https://iss.com", ""))
	assert.Equal(t, &url.URL{Scheme: "http", Host: "iss.com"}, issToURL("http://iss.com", ""))
	assert.Equal(t, &url.URL{Scheme: "https", Host: "iss.com", Path: ""}, issToURL("https://iss.com/", ""))
	assert.Equal(t, &url.URL{Scheme: "https", Host: "iss.com", Path: "/hoge"}, issToURL("https://iss.com/hoge", ""))
	assert.Equal(t, &url.URL{Scheme: "https", Host: "iss.com", Path: "/hoge/foobar"}, issToURL("https://iss.com/hoge", "foobar"))
}

// mockAuthenticator is a test helper that implements the Authenticator interface
type mockAuthenticator struct {
	resendVerificationEmailCalled bool
	resendVerificationEmailUserID string
	resendVerificationEmailError  error
}

func (m *mockAuthenticator) UpdateUser(ctx context.Context, p gateway.AuthenticatorUpdateUserParam) (gateway.AuthenticatorUser, error) {
	return gateway.AuthenticatorUser{}, nil
}

func (m *mockAuthenticator) ResendVerificationEmail(ctx context.Context, userID string) error {
	m.resendVerificationEmailCalled = true
	m.resendVerificationEmailUserID = userID
	return m.resendVerificationEmailError
}

func TestUser_CreateVerification(t *testing.T) {
	user.DefaultPasswordEncoder = &user.NoopPasswordEncoder{}
	uid2 := id.NewUserID()
	uid3 := id.NewUserID()
	tid := id.NewWorkspaceID()
	mocktime := time.Time{}
	mockcode := "CODECODE"

	tests := []struct {
		name                              string
		createUserBefore                  func() *user.User
		email                             string
		authenticatorError                error
		wantError                         error
		wantAuthenticatorCalled           bool
		wantAuthenticatorCalledWithUserID string
	}{
		{
			name: "user without auth0",
			createUserBefore: func() *user.User {
				return user.New().
					ID(id.NewUserID()).
					Workspace(id.NewWorkspaceID()).
					Email("aaa@bbb.com").
					Name("NAME").
					Verification(user.VerificationFrom(mockcode, mocktime, false)).
					MustBuild()
			},
			email:                   "aaa@bbb.com",
			wantError:               nil,
			wantAuthenticatorCalled: false,
		},
		{
			name: "user with auth0",
			createUserBefore: func() *user.User {
				return user.New().
					ID(uid2).
					Workspace(tid).
					Email("auth0user@bbb.com").
					Name("AUTH0USER").
					Auths([]user.Auth{{Provider: "auth0", Sub: "auth0|123456"}}).
					Verification(user.VerificationFrom(mockcode, mocktime, false)).
					MustBuild()
			},
			email:                             "auth0user@bbb.com",
			wantError:                         nil,
			wantAuthenticatorCalled:           true,
			wantAuthenticatorCalledWithUserID: "auth0|123456",
		},
		{
			name: "user with auth0 and reearth",
			createUserBefore: func() *user.User {
				return user.New().
					ID(uid3).
					Workspace(tid).
					Email("mixeduser@bbb.com").
					Name("MIXEDUSER").
					Auths([]user.Auth{
						{Provider: "reearth", Sub: "reearth|abc"},
						{Provider: "auth0", Sub: "auth0|789"},
					}).
					Verification(user.VerificationFrom(mockcode, mocktime, false)).
					MustBuild()
			},
			email:                             "mixeduser@bbb.com",
			wantError:                         nil,
			wantAuthenticatorCalled:           true,
			wantAuthenticatorCalledWithUserID: "auth0|789",
		},
		{
			name: "verified user with auth0 - should skip",
			createUserBefore: func() *user.User {
				return user.New().
					ID(uid2).
					Workspace(tid).
					Email("verified@bbb.com").
					Name("VERIFIED").
					Auths([]user.Auth{{Provider: "auth0", Sub: "auth0|verified"}}).
					Verification(user.VerificationFrom(mockcode, mocktime, true)).
					MustBuild()
			},
			email:                   "verified@bbb.com",
			wantError:               nil,
			wantAuthenticatorCalled: false,
		},
		{
			name: "authenticator returns error",
			createUserBefore: func() *user.User {
				return user.New().
					ID(uid2).
					Workspace(tid).
					Email("erroruser@bbb.com").
					Name("ERRORUSER").
					Auths([]user.Auth{{Provider: "auth0", Sub: "auth0|error"}}).
					Verification(user.VerificationFrom(mockcode, mocktime, false)).
					MustBuild()
			},
			email:                             "erroruser@bbb.com",
			authenticatorError:                rerror.NewE(i18n.T("failed to resend verification email")),
			wantError:                         rerror.NewE(i18n.T("failed to resend verification email")),
			wantAuthenticatorCalled:           true,
			wantAuthenticatorCalledWithUserID: "auth0|error",
		},
		{
			name: "verified user",
			createUserBefore: func() *user.User {
				return user.New().
					ID(id.NewUserID()).
					Workspace(id.NewWorkspaceID()).
					Email("aaa@bbb.com").
					Name("NAME").
					Verification(user.VerificationFrom(mockcode, mocktime, true)).
					MustBuild()
			},
			email:                   "aaa@bbb.com",
			wantError:               nil,
			wantAuthenticatorCalled: false,
		},
		{
			name: "verification not expired - should return nil without creating new verification",
			createUserBefore: func() *user.User {
				return user.New().
					ID(id.NewUserID()).
					Workspace(id.NewWorkspaceID()).
					Email("notexpired@bbb.com").
					Name("NOTEXPIRED").
					Verification(user.VerificationFrom(mockcode, time.Now().Add(1*time.Hour), false)).
					MustBuild()
			},
			email:                   "notexpired@bbb.com",
			wantError:               nil,
			wantAuthenticatorCalled: false,
		},
		{
			name:                    "not found",
			email:                   "ccc@bbb.com",
			wantError:               rerror.ErrNotFound,
			wantAuthenticatorCalled: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			r := accountmemory.New()

			if tt.createUserBefore != nil {
				assert.NoError(t, r.User.Save(ctx, tt.createUserBefore()))
			}

			m := mailer.NewMock()
			auth := &mockAuthenticator{
				resendVerificationEmailError: tt.authenticatorError,
			}
			g := &gateway.Container{
				Mailer:        m,
				Authenticator: auth,
			}
			uc := NewUser(r, g, "", "")

			err := uc.CreateVerification(ctx, tt.email)

			if tt.wantError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				user, err := r.User.FindByEmail(ctx, tt.email)
				assert.NoError(t, err)
				assert.NotNil(t, user.Verification())
			}

			assert.Equal(t, tt.wantAuthenticatorCalled, auth.resendVerificationEmailCalled, "authenticator call mismatch")
			if tt.wantAuthenticatorCalled {
				assert.Equal(t, tt.wantAuthenticatorCalledWithUserID, auth.resendVerificationEmailUserID, "authenticator userID mismatch")
			}
		})
	}
}
