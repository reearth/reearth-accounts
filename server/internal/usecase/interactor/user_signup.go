package interactor

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	htmlTmpl "html/template"
	"net/http"
	"net/url"
	"path"
	"strings"
	textTmpl "text/template"

	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/log"
	"github.com/reearth/reearthx/mailer"
	"github.com/reearth/reearthx/rerror"
	"github.com/samber/lo"
)

type mailContent struct {
	UserName    string
	Message     string
	Suffix      string
	ActionLabel string
	ActionURL   htmlTmpl.URL
}

type OpenIDConfiguration struct {
	UserinfoEndpoint string `json:"userinfo_endpoint"`
}

type UserInfo struct {
	Sub      string `json:"sub"`
	Name     string `json:"name"`
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
	Error    string `json:"error"`
}

var (
	//go:embed emails/auth_html.tmpl
	autHTMLTMPLStr string
	//go:embed emails/auth_text.tmpl
	authTextTMPLStr string

	authTextTMPL *textTmpl.Template
	authHTMLTMPL *htmlTmpl.Template
)

func init() {
	var err error
	authTextTMPL, err = textTmpl.New("passwordReset").Parse(authTextTMPLStr)
	if err != nil {
		log.Panicf("password reset email template parse error: %s\n", err)
	}
	authHTMLTMPL, err = htmlTmpl.New("passwordReset").Parse(autHTMLTMPLStr)
	if err != nil {
		log.Panicf("password reset email template parse error: %s\n", err)
	}
}

func (i *User) Signup(ctx context.Context, param interfaces.SignupParam) (u *user.User, err error) {
	if err = i.verifySignupSecret(param.Secret); err != nil {
		return nil, err
	}

	return Run1(ctx, nil, i.repos, Usecase().Transaction(), func(ctx context.Context) (*user.User, error) {
		log.Debugfc(ctx, "[Signup] Inside transaction")
		// Check for duplicate email
		eu, err := i.repos.User.FindByEmail(ctx, param.Email)
		if err != nil && !errors.Is(err, rerror.ErrNotFound) {
			log.Errorfc(ctx, "[Signup] Error finding user by email: %v", err)
			return nil, err
		}
		if eu != nil {
			log.Warnfc(ctx, "[Signup] User already exists with email: %s", param.Email)
			return nil, interfaces.ErrUserAlreadyExists
		}

		u, ws, err := workspace.Init(workspace.InitParams{
			Email:       param.Email,
			Name:        param.Name,
			Password:    lo.ToPtr(param.Password),
			Lang:        param.Lang,
			Theme:       param.Theme,
			UserID:      param.UserID,
			WorkspaceID: param.WorkspaceID,
		})
		if err != nil {
			return nil, err
		}

		vr := user.NewVerification()
		u.SetVerification(vr)

		if err = i.repos.User.Create(ctx, u); err != nil {
			if errors.Is(err, repo.ErrDuplicatedUser) {
				return nil, interfaces.ErrUserAlreadyExists
			}
			return nil, err
		}
		if err = i.repos.Workspace.Save(ctx, ws); err != nil {
			if errors.Is(err, repo.ErrDuplicatedUser) {
				return nil, interfaces.ErrUserAliasAlreadyExists
			}
			if errors.Is(err, repo.ErrDuplicateWorkspaceAlias) {
				return nil, interfaces.ErrWorkspaceAliasAlreadyExists
			}
			return nil, err
		}

		// Find or create required roles (auto-create when MockAuth is enabled)
		roleSelf, err := i.findOrCreateRole(ctx, interfaces.RoleSelf, param.MockAuth)
		if err != nil {
			return nil, err
		}

		roleOwner, err := i.findOrCreateRole(ctx, role.RoleOwner.String(), param.MockAuth)
		if err != nil {
			return nil, err
		}

		wsRole := permittable.NewWorkspaceRole(ws.ID(), roleOwner.ID())
		perm := permittable.New().NewID().RoleIDs([]id.RoleID{roleSelf.ID()}).UserID(u.ID()).WorkspaceRoles([]permittable.WorkspaceRole{wsRole}).MustBuild()
		if err = i.repos.Permittable.Save(ctx, lo.FromPtr(perm)); err != nil {
			return nil, err
		}

		if !param.MockAuth {
			if err = i.sendVerificationMail(ctx, u, vr); err != nil {
				return nil, err
			}
		}

		return u, nil
	})
}

func (i *User) SignupOIDC(ctx context.Context, param interfaces.SignupOIDCParam) (*user.User, error) {
	if err := i.verifySignupSecret(param.Secret); err != nil {
		return nil, err
	}

	sub := param.Sub
	name := param.Name
	email := param.Email
	if sub == "" || email == "" {
		ui, err := getUserInfoFromISS(ctx, param.Issuer, param.AccessToken)
		if err != nil {
			return nil, err
		}
		sub = ui.Sub
		email = ui.Email
	}

	return Run1(ctx, nil, i.repos, Usecase().Transaction(), func(ctx context.Context) (*user.User, error) {
		eu, err := i.repos.User.FindByEmail(ctx, param.Email)
		if err != nil && !errors.Is(err, rerror.ErrNotFound) {
			return nil, err
		}
		if eu != nil {
			return nil, repo.ErrDuplicatedUser
		}

		eu, err = i.repos.User.FindBySub(ctx, param.Sub)
		if err != nil && !errors.Is(err, rerror.ErrNotFound) {
			return nil, err
		}
		if eu != nil {
			return nil, repo.ErrDuplicatedUser
		}

		// Initialize user and ws
		u, ws, err := workspace.Init(workspace.InitParams{
			Email:       email,
			Name:        name,
			Sub:         user.AuthFrom(sub).Ref(),
			Lang:        param.User.Lang,
			Theme:       param.User.Theme,
			UserID:      param.User.UserID,
			WorkspaceID: param.User.WorkspaceID,
		})
		if err != nil {
			return nil, err
		}

		if err = i.repos.User.Create(ctx, u); err != nil {
			return nil, err
		}

		if err = i.repos.Workspace.Save(ctx, ws); err != nil {
			return nil, err
		}

		// Find or create required roles (auto-create when MockAuth is enabled)
		// Note: SignupOIDC doesn't have MockAuth param, so roles must exist in DB
		roleSelf, err := i.repos.Role.FindByName(ctx, interfaces.RoleSelf)
		if err != nil {
			return nil, err
		}

		roleOwner, err := i.repos.Role.FindByName(ctx, role.RoleOwner.String())
		if err != nil {
			return nil, err
		}

		wsRole := permittable.NewWorkspaceRole(ws.ID(), roleOwner.ID())
		perm := permittable.New().NewID().RoleIDs([]id.RoleID{roleSelf.ID()}).UserID(u.ID()).WorkspaceRoles([]permittable.WorkspaceRole{wsRole}).MustBuild()
		if err = i.repos.Permittable.Save(ctx, lo.FromPtr(perm)); err != nil {
			return nil, err
		}

		return u, nil
	})
}

func (i *User) FindOrCreate(ctx context.Context, param interfaces.UserFindOrCreateParam) (u *user.User, err error) {
	return Run1(ctx, nil, i.repos, Usecase().Transaction(), func(ctx context.Context) (*user.User, error) {
		if param.Sub == "" {
			return nil, rerror.ErrNotFound
		}

		// Check if user already exists
		existedUser, err := i.repos.User.FindBySub(ctx, param.Sub)
		if err != nil && !errors.Is(err, rerror.ErrNotFound) {
			return nil, err
		} else if existedUser != nil {
			return existedUser, nil
		}

		ui, err := getUserInfoFromISS(ctx, param.ISS, param.Token)
		if err != nil {
			return nil, err
		}

		u, workspace, err := workspace.Init(workspace.InitParams{
			Email: ui.Email,
			Name:  ui.Name,
			Sub:   user.AuthFrom(param.Sub).Ref(),
		})
		if err != nil {
			return nil, err
		}

		u2, err := i.repos.User.FindBySubOrCreate(ctx, u, param.Sub)
		if err != nil {
			return nil, err
		}

		if err := i.repos.Workspace.Save(ctx, workspace); err != nil {
			return nil, err
		}

		return u2, nil
	})
}

func (i *User) sendVerificationMail(ctx context.Context, u *user.User, vr *user.Verification) error {
	var text, html bytes.Buffer
	link := i.authSrvUIDomain + "/?user-verification-token=" + vr.Code()
	signupMailContent := mailContent{
		Message:     "Thank you for signing up to Re:Earth. Please verify your email address by clicking the button below.",
		Suffix:      "You can use this email address to log in to Re:Earth account anytime.",
		ActionLabel: "Activate your account and log in",
		UserName:    u.Email(),
		ActionURL:   htmlTmpl.URL(link),
	}
	if err := authTextTMPL.Execute(&text, signupMailContent); err != nil {
		return err
	}
	if err := authHTMLTMPL.Execute(&html, signupMailContent); err != nil {
		return err
	}

	if err := i.gateways.Mailer.SendMail(
		ctx,
		[]mailer.Contact{
			{
				Email: u.Email(),
				Name:  u.Name(),
			},
		},
		"email verification",
		text.String(),
		html.String(),
	); err != nil {
		return err
	}

	return nil
}

func getUserInfoFromISS(ctx context.Context, iss, accessToken string) (UserInfo, error) {
	if accessToken == "" {
		return UserInfo{}, rerror.NewE(i18n.T("invalid access token"))
	}
	if iss == "" {
		return UserInfo{}, rerror.NewE(i18n.T("invalid issuer"))
	}

	var u string
	c, err := getOpenIDConfiguration(ctx, iss)
	if err != nil {
		u2 := issToURL(iss, "/userinfo")
		if u2 == nil {
			return UserInfo{}, rerror.NewE(i18n.T("invalid iss"))
		}
		u = u2.String()
	} else {
		u = c.UserinfoEndpoint
	}
	return getUserInfo(ctx, u, accessToken)
}

func getOpenIDConfiguration(ctx context.Context, iss string) (c OpenIDConfiguration, err error) {
	WKUrl := issToURL(iss, "/.well-known/openid-configuration")
	if WKUrl == nil {
		err = rerror.NewE(i18n.T("invalid iss"))
		return
	}

	if ctx == nil {
		ctx = context.Background()
	}

	req, err2 := http.NewRequestWithContext(ctx, http.MethodGet, WKUrl.String(), nil)
	if err2 != nil {
		err = err2
		return
	}

	res, err2 := http.DefaultClient.Do(req)
	if err2 != nil {
		err = err2
		return
	}

	if res.StatusCode != http.StatusOK {
		err = rerror.NewE(i18n.T("could not get user info"))
		return
	}

	if err2 := json.NewDecoder(res.Body).Decode(&c); err2 != nil {
		err = fmt.Errorf("could not get user info: %w", err2)
		return
	}

	return
}

func getUserInfo(ctx context.Context, url, accessToken string) (ui UserInfo, err error) {
	if ctx == nil {
		ctx = context.Background()
	}

	req, err2 := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err2 != nil {
		err = err2
		return
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	res, err2 := http.DefaultClient.Do(req)
	if err2 != nil {
		err = err2
		return
	}

	if res.StatusCode != http.StatusOK {
		err = rerror.NewE(i18n.T("could not get user info"))
		return
	}

	if err2 := json.NewDecoder(res.Body).Decode(&ui); err2 != nil {
		err = fmt.Errorf("could not get user info: %w", err2)
		return
	}

	if ui.Error != "" {
		err = fmt.Errorf("could not get user info: %s", ui.Error)
		return
	}
	if ui.Sub == "" {
		err = fmt.Errorf("could not get user info: invalid response")
		return
	}
	if ui.Email == "" {
		err = fmt.Errorf("could not get user info: email scope missing")
		return
	}

	return
}

func issToURL(iss, p string) *url.URL {
	if iss == "" {
		return nil
	}

	if !strings.HasPrefix(iss, "https://") && !strings.HasPrefix(iss, "http://") {
		iss = "https://" + iss
	}

	u, err := url.Parse(iss)
	if err == nil {
		u.Path = path.Join(u.Path, p)
		if u.Path == "/" {
			u.Path = ""
		}
		return u
	}

	return nil
}

func (i *User) verifySignupSecret(secret *string) error {
	if i.signupSecret != "" && (secret == nil || *secret != i.signupSecret) {
		return interfaces.ErrSignupInvalidSecret
	}
	return nil
}

// findOrCreateRole finds a role by name, or creates it if not found and mockAuth is enabled
func (i *User) findOrCreateRole(ctx context.Context, roleName string, mockAuth bool) (*role.Role, error) {
	r, err := i.repos.Role.FindByName(ctx, roleName)
	if err == nil {
		return r, nil
	}

	// If role not found and mockAuth is enabled, create it
	if errors.Is(err, rerror.ErrNotFound) && mockAuth {
		log.Infof("[MockAuth] Auto-creating role: %s", roleName)
		newRole := role.New().NewID().Name(roleName).MustBuild()
		if err := i.repos.Role.Save(ctx, *newRole); err != nil {
			return nil, fmt.Errorf("failed to auto-create role %s: %w", roleName, err)
		}
		return newRole, nil
	}

	return nil, err
}

func (i *User) CreateVerification(ctx context.Context, email string) error {
	return Run0(ctx, nil, i.repos, Usecase().Transaction(), func(ctx context.Context) error {
		u, err := i.repos.User.FindByEmail(ctx, email)
		if err != nil {
			return err
		}

		if u.Verification().IsVerified() {
			log.Warnc(ctx, "user is already verified")
			return nil
		}

		if !u.Verification().IsExpired() {
			log.Warnc(ctx, "user verification is not expired")
			return nil
		}

		vr := user.NewVerification()
		u.SetVerification(vr)

		if err = i.repos.User.Save(ctx, u); err != nil {
			return err
		}

		auth0Auth := u.Auths().GetByProvider("auth0")
		if auth0Auth != nil {
			if err = i.gateways.Authenticator.ResendVerificationEmail(ctx, auth0Auth.Sub); err != nil {
				return err
			}
		}

		return nil
	})
}
