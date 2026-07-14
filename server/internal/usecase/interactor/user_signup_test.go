package interactor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/samber/lo"
	"golang.org/x/text/language"

	accountmemory "github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
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
			name:            "duplicate user alias",
			signupSecret:    "",
			authSrvUIDomain: "",
			createUserBefore: user.New().
				ID(uid).
				Workspace(tid).
				Name("NAME").
				Alias("NAME").
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
			wantError:     interfaces.ErrUserAlreadyExists,
		},
		{
			name:             "duplicate workspace alias - memory repo allows",
			signupSecret:     "",
			authSrvUIDomain:  "",
			createUserBefore: nil,
			args: interfaces.SignupParam{
				Email:       "unique@bbb.com",
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
					Alias("NAME").
					Auths(u.Auths()).
					Metadata(*u.Metadata()).
					Email("unique@bbb.com").
					PasswordPlainText("PAss00!!").
					Verification(user.VerificationFrom(mockcode, mocktime.Add(24*time.Hour), false)).
					MustBuild()
			},
			wantWorkspace: workspace.New().
				ID(tid).
				Name("NAME").
				Alias("NAME").
				Members(map[user.ID]workspace.Member{uid: {Role: role.RoleOwner, Disabled: false, InvitedBy: uid}}).
				Personal(true).
				MustBuild(),
			wantMailTo:      []mailer.Contact{{Email: "unique@bbb.com", Name: "NAME"}},
			wantMailSubject: "email verification",
			wantMailContent: "/?user-verification-token=CODECODE",
			wantError:       nil,
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
					Alias("NAME").
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
				Alias("NAME").
				Members(map[user.ID]workspace.Member{uid: {Role: role.RoleOwner, Disabled: false, InvitedBy: uid}}).
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
					Alias("NAME").
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
				Alias("NAME").
				Members(map[user.ID]workspace.Member{uid: {Role: role.RoleOwner, Disabled: false, InvitedBy: uid}}).
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
					Alias("NAME").
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
				Alias("NAME").
				Members(map[user.ID]workspace.Member{uid: {Role: role.RoleOwner, Disabled: false, InvitedBy: uid}}).
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

			// Create required roles for signup
			selfRole := role.New().NewID().Name(interfaces.RoleSelf).MustBuild()
			ownerRole := role.New().NewID().Name(role.RoleOwner.String()).MustBuild()
			assert.NoError(t, r.Role.Save(ctx, *selfRole))
			assert.NoError(t, r.Role.Save(ctx, *ownerRole))

			if tt.createUserBefore != nil {
				assert.NoError(t, r.User.Save(ctx, tt.createUserBefore))
			}

			m := mailer.NewMock()
			g := &gateway.Container{Mailer: m}
			uc := NewUser(r, g, tt.signupSecret, tt.authSrvUIDomain)
			u, err := uc.Signup(ctx, tt.args)

			if tt.wantUser != nil {
				expectedUser := tt.wantUser(u)
				assert.NotNil(t, u)
				assert.Equal(t, expectedUser.ID(), u.ID())
				assert.Equal(t, expectedUser.Name(), u.Name())
				assert.Equal(t, expectedUser.Alias(), u.Alias())
				assert.Equal(t, expectedUser.Email(), u.Email())
				assert.Equal(t, expectedUser.Workspace(), u.Workspace())
				assert.Equal(t, expectedUser.Auths(), u.Auths())
				assert.Equal(t, expectedUser.Metadata(), u.Metadata())
				assert.Equal(t, expectedUser.Verification(), u.Verification())
				assert.NotZero(t, u.UpdatedAt(), "updatedAt should be set")
			} else {
				assert.Nil(t, u)
			}

			var ws *workspace.Workspace
			if u != nil {
				ws, _ = r.Workspace.FindByID(ctx, u.Workspace())
			}
			if tt.wantWorkspace != nil {
				assert.NotNil(t, ws)
				assert.Equal(t, tt.wantWorkspace.ID(), ws.ID())
				assert.Equal(t, tt.wantWorkspace.Name(), ws.Name())
				assert.Equal(t, tt.wantWorkspace.Alias(), ws.Alias())
				assert.Equal(t, tt.wantWorkspace.Members(), ws.Members())
				assert.NotZero(t, ws.UpdatedAt(), "updatedAt should be set")
			} else {
				assert.Nil(t, ws)
			}

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

func TestGetOpenIDConfiguration(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		iss        string
		wantErr    bool
		wantResult OpenIDConfiguration
	}{
		{
			name: "success",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"userinfo_endpoint":"https://example.com/userinfo"}`))
			},
			wantResult: OpenIDConfiguration{UserinfoEndpoint: "https://example.com/userinfo"},
		},
		{
			name: "non-200 response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("internal server error"))
			},
			wantErr: true,
		},
		{
			name: "invalid JSON",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("not valid json"))
			},
			wantErr: true,
		},
		{
			name:    "empty issuer",
			iss:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			iss := tt.iss
			if tt.handler != nil {
				srv := httptest.NewServer(tt.handler)
				defer srv.Close()
				iss = srv.URL
			}

			result, err := getOpenIDConfiguration(context.Background(), iss)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantResult, result)
			}
		})
	}
}

func TestGetUserInfo(t *testing.T) {
	tests := []struct {
		name        string
		handler     http.HandlerFunc
		accessToken string
		wantErr     bool
		wantResult  UserInfo
	}{
		{
			name: "success",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"sub":"sub123","email":"user@example.com","name":"Test User"}`))
			},
			accessToken: "token123",
			wantResult:  UserInfo{Sub: "sub123", Email: "user@example.com", Name: "Test User"},
		},
		{
			name: "authorization header forwarded",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer mytoken" {
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"sub":"sub456","email":"other@example.com"}`))
			},
			accessToken: "mytoken",
			wantResult:  UserInfo{Sub: "sub456", Email: "other@example.com"},
		},
		{
			name: "non-200 response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte("forbidden"))
			},
			accessToken: "token123",
			wantErr:     true,
		},
		{
			name: "invalid JSON",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("not valid json"))
			},
			accessToken: "token123",
			wantErr:     true,
		},
		{
			name: "error field in response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"error":"access_denied"}`))
			},
			accessToken: "token123",
			wantErr:     true,
		},
		{
			name: "missing sub",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"email":"user@example.com"}`))
			},
			accessToken: "token123",
			wantErr:     true,
		},
		{
			name: "missing email",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"sub":"sub123"}`))
			},
			accessToken: "token123",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(tt.handler)
			defer srv.Close()

			result, err := getUserInfo(context.Background(), srv.URL, tt.accessToken)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantResult, result)
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

func TestUser_getUserInfoFromISS_AllowlistValidation(t *testing.T) {
	tests := []struct {
		name        string
		allowedISS  []string
		iss         string
		accessToken string
		wantErrMsg  string
	}{
		{
			name:        "empty access token",
			allowedISS:  nil,
			iss:         "https://example.com",
			accessToken: "",
			wantErrMsg:  "invalid access token",
		},
		{
			name:        "empty iss",
			allowedISS:  nil,
			iss:         "",
			accessToken: "token",
			wantErrMsg:  "invalid issuer",
		},
		{
			name:        "iss not in allowlist",
			allowedISS:  []string{"https://trusted.example.com"},
			iss:         "https://evil.example.com",
			accessToken: "token",
			wantErrMsg:  "invalid issuer",
		},
		{
			name:        "second iss not in allowlist",
			allowedISS:  []string{"https://a.example.com", "https://b.example.com"},
			iss:         "https://c.example.com",
			accessToken: "token",
			wantErrMsg:  "invalid issuer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &User{allowedISS: tt.allowedISS}
			_, err := u.getUserInfoFromISS(context.Background(), tt.iss, tt.accessToken)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErrMsg)
		})
	}
}

func TestUser_getUserInfoFromISS_AllowlistPermits(t *testing.T) {
	tests := []struct {
		name       string
		allowedISS []string
		iss        string
	}{
		{
			name:       "empty allowlist permits any iss",
			allowedISS: nil,
			iss:        "https://any.example.com",
		},
		{
			name:       "iss present in single-entry allowlist",
			allowedISS: []string{"https://trusted.example.com"},
			iss:        "https://trusted.example.com",
		},
		{
			name:       "iss present in multi-entry allowlist",
			allowedISS: []string{"https://a.example.com", "https://b.example.com"},
			iss:        "https://b.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &User{allowedISS: tt.allowedISS}
			// The ISS passes the allowlist check; error comes from the unreachable host,
			// not from the "invalid issuer" guard.
			_, err := u.getUserInfoFromISS(context.Background(), tt.iss, "token")
			assert.Error(t, err)
			assert.NotContains(t, err.Error(), "invalid issuer")
		})
	}
}

func TestUser_getUserInfoFromISS_MockServer(t *testing.T) {
	var serverURL string

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(OpenIDConfiguration{
			UserinfoEndpoint: serverURL + "/userinfo",
		})
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer mytoken", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(UserInfo{
			Sub:   "sub123",
			Name:  "Test User",
			Email: "test@example.com",
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	serverURL = srv.URL

	u := &User{allowedISS: []string{srv.URL}}
	info, err := u.getUserInfoFromISS(context.Background(), srv.URL, "mytoken")

	assert.NoError(t, err)
	assert.Equal(t, "sub123", info.Sub)
	assert.Equal(t, "test@example.com", info.Email)
	assert.Equal(t, "Test User", info.Name)
}

func TestUser_getUserInfoFromISS_ContextDeadlinePropagated(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	u := &User{}
	_, err := u.getUserInfoFromISS(ctx, srv.URL, "token")

	assert.Error(t, err)
}

func TestUser_SignupOIDC_AllowlistRejectsUnknownISS(t *testing.T) {
	ctx := context.Background()
	r := accountmemory.New()

	uc := NewUser(r, nil, "", "", "https://trusted.example.com")

	_, err := uc.SignupOIDC(ctx, interfaces.SignupOIDCParam{
		Issuer:      "https://evil.example.com",
		AccessToken: "sometoken",
		// Sub and Email intentionally empty to trigger getUserInfoFromISS
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid issuer")
}

func TestUser_SignupOIDC_AllowlistPermitsKnownISS(t *testing.T) {
	var serverURL string

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(OpenIDConfiguration{
			UserinfoEndpoint: serverURL + "/userinfo",
		})
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(UserInfo{
			Sub:   "sub-signup",
			Email: "oidcuser@example.com",
			Name:  "OIDC User",
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	serverURL = srv.URL

	ctx := context.Background()
	r := accountmemory.New()

	selfRole := role.New().NewID().Name(interfaces.RoleSelf).MustBuild()
	ownerRole := role.New().NewID().Name(role.RoleOwner.String()).MustBuild()
	assert.NoError(t, r.Role.Save(ctx, *selfRole))
	assert.NoError(t, r.Role.Save(ctx, *ownerRole))

	uc := NewUser(r, nil, "", "", srv.URL)

	u, err := uc.SignupOIDC(ctx, interfaces.SignupOIDCParam{
		Issuer:      srv.URL,
		AccessToken: "token",
	})

	assert.NoError(t, err)
	assert.NotNil(t, u)
	assert.Equal(t, "oidcuser@example.com", u.Email())
}

func TestUser_FindOrCreate_AllowlistRejectsUnknownISS(t *testing.T) {
	ctx := context.Background()
	r := accountmemory.New()

	uc := NewUser(r, nil, "", "", "https://trusted.example.com").(*User)

	_, err := uc.FindOrCreate(ctx, interfaces.UserFindOrCreateParam{
		Sub:   "sub123",
		ISS:   "https://evil.example.com",
		Token: "sometoken",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid issuer")
}

// mockAuthenticator is a test helper that implements the Authenticator interface
type mockAuthenticator struct {
	resendVerificationEmailCalled bool
	resendVerificationEmailUserID string
	resendVerificationEmailError  error
	updateUserCalled              bool
	updateUserParam               gateway.AuthenticatorUpdateUserParam
	updateUserError               error
}

func (m *mockAuthenticator) UpdateUser(ctx context.Context, p gateway.AuthenticatorUpdateUserParam) (gateway.AuthenticatorUser, error) {
	m.updateUserCalled = true
	m.updateUserParam = p
	if m.updateUserError != nil {
		return gateway.AuthenticatorUser{}, m.updateUserError
	}
	return gateway.AuthenticatorUser{
		ID:            p.ID,
		Name:          lo.FromPtr(p.Name),
		Email:         lo.FromPtr(p.Email),
		EmailVerified: true,
	}, nil
}

func (m *mockAuthenticator) DisableMFA(_ context.Context, _ string) error { return nil }

func (m *mockAuthenticator) EnableMFA(_ context.Context, _ string) (string, error) { return "", nil }

func (m *mockAuthenticator) GetMFAStatus(_ context.Context, _ string) (gateway.MFAStatus, error) {
	return gateway.MFAStatus{}, nil
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
				Mailer:         m,
				Authenticators: map[gateway.Provider]gateway.Authenticator{gateway.ProviderAuth0: auth},
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

func TestUser_SyncSSOUser(t *testing.T) {
	uid := id.NewUserID()
	wid := id.NewWorkspaceID()
	const sub = "samlp|org123|idp456"

	setupRoles := func(ctx context.Context, r *repo.Container) {
		selfRole := role.New().NewID().Name(interfaces.RoleSelf).MustBuild()
		ownerRole := role.New().NewID().Name(role.RoleOwner.String()).MustBuild()
		_ = r.Role.Save(ctx, *selfRole)
		_ = r.Role.Save(ctx, *ownerRole)
	}

	t.Run("creates new user", func(t *testing.T) {
		ctx := context.Background()
		r := accountmemory.New()
		setupRoles(ctx, r)

		uc := NewUser(r, nil, "", "")
		u, err := uc.SyncSSOUser(ctx, interfaces.SyncSSOUserParam{
			Email:       "sso@example.com",
			Name:        "SSO User",
			Sub:         sub,
			UserID:      &uid,
			WorkspaceID: &wid,
		})

		assert.NoError(t, err)
		assert.NotNil(t, u)
		assert.Equal(t, "sso@example.com", u.Email())
		assert.Equal(t, "SSO User", u.Name())
	})

	t.Run("idempotent: returns existing user when sub already registered", func(t *testing.T) {
		ctx := context.Background()
		r := accountmemory.New()
		setupRoles(ctx, r)

		uc := NewUser(r, nil, "", "")
		first, err := uc.SyncSSOUser(ctx, interfaces.SyncSSOUserParam{
			Email:       "sso@example.com",
			Name:        "SSO User",
			Sub:         sub,
			UserID:      &uid,
			WorkspaceID: &wid,
		})
		assert.NoError(t, err)

		second, err := uc.SyncSSOUser(ctx, interfaces.SyncSSOUserParam{
			Email:       "sso@example.com",
			Name:        "SSO User",
			Sub:         sub,
		})
		assert.NoError(t, err)
		assert.NotNil(t, second)
		assert.Equal(t, first.ID(), second.ID())
	})

	t.Run("returns error when email already used by different user", func(t *testing.T) {
		ctx := context.Background()
		r := accountmemory.New()
		setupRoles(ctx, r)

		existing := user.New().NewID().Workspace(wid).Name("Existing").Email("taken@example.com").MustBuild()
		assert.NoError(t, r.User.Save(ctx, existing))

		uc := NewUser(r, nil, "", "")
		_, err := uc.SyncSSOUser(ctx, interfaces.SyncSSOUserParam{
			Email: "taken@example.com",
			Name:  "SSO User",
			Sub:   "samlp|org123|other",
		})

		assert.ErrorIs(t, err, interfaces.ErrUserAlreadyExists)
	})

	t.Run("supports optional lang and theme", func(t *testing.T) {
		ctx := context.Background()
		r := accountmemory.New()
		setupRoles(ctx, r)

		uc := NewUser(r, nil, "", "")
		u, err := uc.SyncSSOUser(ctx, interfaces.SyncSSOUserParam{
			Email: "sso2@example.com",
			Name:  "SSO User 2",
			Sub:   "samlp|org123|idp789",
			Lang:  lo.ToPtr(language.Japanese),
			Theme: user.ThemeDark.Ref(),
		})

		assert.NoError(t, err)
		assert.NotNil(t, u)
		assert.Equal(t, "sso2@example.com", u.Email())
	})
}
