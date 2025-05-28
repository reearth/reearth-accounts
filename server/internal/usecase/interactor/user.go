package interactor

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	htmlTmpl "html/template"

	"github.com/reearth/reearth-accounts/server/internal/usecase"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/mailer"
	"github.com/reearth/reearthx/rerror"
	"github.com/samber/lo"
)

type User struct {
	repos           *repo.Container
	gateways        *gateway.Container
	signupSecret    string
	authSrvUIDomain string
	query           interfaces.UserQuery
}

var (
	passwordResetMailContent = mailContent{
		Message:     "Thank you for using Re:Earth. We've received a request to reset your password. If this was you, please click the link below to confirm and change your password.",
		Suffix:      "If you did not mean to reset your password, then you can ignore this email.",
		ActionLabel: "Confirm to reset your password",
	}
)

func NewUser(r *repo.Container, g *gateway.Container, signupSecret, authSrcUIDomain string) interfaces.User {
	var repos []repo.User
	if r != nil {
		repos = []repo.User{r.User}
	}
	return &User{
		repos:           r,
		gateways:        g,
		signupSecret:    signupSecret,
		authSrvUIDomain: authSrcUIDomain,
		query: &UserQuery{
			repos: repos,
		},
	}
}

func NewMultiUser(r *repo.Container, g *gateway.Container, signupSecret, authSrcUIDomain string, users []repo.User) interfaces.User {
	return &User{
		repos:           r,
		gateways:        g,
		signupSecret:    signupSecret,
		authSrvUIDomain: authSrcUIDomain,
		query: &UserQuery{
			repos: append([]repo.User{r.User}, users...),
		},
	}
}

func (i *User) FetchByID(ctx context.Context, ids user.IDList) (user.List, error) {
	return i.query.FetchByID(ctx, ids)
}

func (i *User) FetchBySub(ctx context.Context, sub string) (*user.User, error) {
	return i.query.FetchBySub(ctx, sub)
}

func (i *User) FetchByNameOrEmail(ctx context.Context, nameOrEmail string) (*user.Simple, error) {
	return i.query.FetchByNameOrEmail(ctx, nameOrEmail)
}

func (i *User) SearchUser(ctx context.Context, keyword string) (user.SimpleList, error) {
	return i.query.SearchUser(ctx, keyword)
}

func (i *User) GetUserByCredentials(ctx context.Context, inp interfaces.GetUserByCredentials) (u *user.User, err error) {
	return Run1(ctx, nil, i.repos, Usecase().Transaction(), func(ctx context.Context) (*user.User, error) {
		u, err = i.repos.User.FindByNameOrEmail(ctx, inp.Email)
		if err != nil && !errors.Is(err, rerror.ErrNotFound) {
			return nil, err
		} else if u == nil {
			return nil, interfaces.ErrInvalidUserEmail
		}
		matched, err := u.MatchPassword(inp.Password)
		if err != nil {
			return nil, err
		}
		if !matched {
			return nil, interfaces.ErrInvalidEmailOrPassword
		}
		if u.Verification() == nil || !u.Verification().IsVerified() {
			return nil, interfaces.ErrNotVerifiedUser
		}
		return u, nil
	})
}

func (i *User) GetUserBySubject(ctx context.Context, sub string) (u *user.User, err error) {
	return Run1(ctx, nil, i.repos, Usecase().Transaction(), func(ctx context.Context) (*user.User, error) {
		u, err = i.repos.User.FindBySub(ctx, sub)
		if err != nil {
			return nil, err
		}
		return u, nil
	})
}

func (i *User) UpdateMe(ctx context.Context, p interfaces.UpdateMeParam, operator *usecase.Operator) (u *user.User, err error) {
	if operator.User == nil {
		return nil, interfaces.ErrInvalidOperator
	}

	return Run1(ctx, operator, i.repos, Usecase().Transaction(), func(ctx context.Context) (*user.User, error) {
		if p.Password != nil {
			if p.PasswordConfirmation == nil || *p.Password != *p.PasswordConfirmation {
				return nil, interfaces.ErrUserInvalidPasswordConfirmation
			}
		}

		var workspace *workspace.Workspace

		u, err = i.repos.User.FindByID(ctx, *operator.User)
		if err != nil {
			return nil, err
		}

		if p.Name != nil && *p.Name != u.Name() {
			oldName := u.Name()
			u.UpdateName(*p.Name)

			workspace, err = i.repos.Workspace.FindByID(ctx, u.Workspace())
			if err != nil && !errors.Is(err, rerror.ErrNotFound) {
				return nil, err
			}

			tn := workspace.Name()
			if tn == "" || tn == oldName {
				workspace.Rename(*p.Name)
			} else {
				workspace = nil
			}
		}
		if p.Email != nil {
			if err := u.UpdateEmail(*p.Email); err != nil {
				return nil, err
			}
		}
		if p.Lang != nil {
			u.UpdateLang(*p.Lang)
		}
		if p.Theme != nil {
			u.UpdateTheme(*p.Theme)
		}

		if p.Password != nil && u.HasAuthProvider("reearth") {
			if err := u.SetPassword(*p.Password); err != nil {
				return nil, err
			}
		}

		// Update Auth0 users
		if p.Name != nil || p.Email != nil || p.Password != nil {
			for _, a := range u.Auths() {
				if a.Provider != "auth0" {
					continue
				}
				if _, err := i.gateways.Authenticator.UpdateUser(ctx, gateway.AuthenticatorUpdateUserParam{
					ID:       a.Sub,
					Name:     p.Name,
					Email:    p.Email,
					Password: p.Password,
				}); err != nil {
					return nil, err
				}
			}
		}

		if workspace != nil {
			err = i.repos.Workspace.Save(ctx, workspace)
			if err != nil {
				return nil, err
			}
		}

		err = i.repos.User.Save(ctx, u)
		if err != nil {
			return nil, err
		}

		return u, nil
	})
}

func (i *User) RemoveMyAuth(ctx context.Context, authProvider string, operator *usecase.Operator) (u *user.User, err error) {
	if operator.User == nil {
		return nil, interfaces.ErrInvalidOperator
	}

	return Run1(ctx, operator, i.repos, Usecase().Transaction(), func(ctx context.Context) (*user.User, error) {
		u, err = i.repos.User.FindByID(ctx, *operator.User)
		if err != nil {
			return nil, err
		}

		u.RemoveAuthByProvider(authProvider)

		err = i.repos.User.Save(ctx, u)
		if err != nil {
			return nil, err
		}

		return u, nil
	})
}

func (i *User) DeleteMe(ctx context.Context, userID user.ID, operator *usecase.Operator) (err error) {
	if operator.User == nil {
		return interfaces.ErrInvalidOperator
	}
	return Run0(ctx, operator, i.repos, Usecase().Transaction(), func(ctx context.Context) error {
		if userID.IsNil() || userID != *operator.User {
			return rerror.NewE(i18n.T("invalid user id"))
		}

		u, err := i.repos.User.FindByID(ctx, userID)
		if err != nil && !errors.Is(err, rerror.ErrNotFound) {
			return err
		}
		if u == nil {
			return nil
		}

		workspaces, err := i.repos.Workspace.FindByUser(ctx, u.ID())
		if err != nil {
			return err
		}

		updatedWorkspaces := make([]*workspace.Workspace, 0, len(workspaces))
		deletedWorkspaces := []user.WorkspaceID{}

		for _, workspace := range workspaces {
			if !workspace.IsPersonal() && !workspace.Members().IsOnlyOwner(u.ID()) {
				_ = workspace.Members().Leave(u.ID())
				updatedWorkspaces = append(updatedWorkspaces, workspace)
				continue
			}

			deletedWorkspaces = append(deletedWorkspaces, workspace.ID())
		}

		// Save workspaces
		if err := i.repos.Workspace.SaveAll(ctx, updatedWorkspaces); err != nil {
			return err
		}

		// Delete workspaces
		if err := i.repos.Workspace.RemoveAll(ctx, deletedWorkspaces); err != nil {
			return err
		}

		// Delete user
		if err := i.repos.User.Remove(ctx, u.ID()); err != nil {
			return err
		}

		return nil
	})

}

func (i *User) VerifyUser(ctx context.Context, code string) (*user.User, error) {
	return Run1(ctx, nil, i.repos, Usecase().Transaction(), func(ctx context.Context) (*user.User, error) {

		u, err := i.repos.User.FindByVerification(ctx, code)
		if err != nil {
			return nil, err
		}
		if u.Verification().IsExpired() {
			return nil, errors.New("verification expired")
		}
		u.Verification().SetVerified(true)
		err = i.repos.User.Save(ctx, u)
		if err != nil {
			return nil, err
		}

		return u, nil
	})
}
func (i *User) StartPasswordReset(ctx context.Context, email string) error {
	return Run0(ctx, nil, i.repos, Usecase().Transaction(), func(ctx context.Context) error {
		u, err := i.repos.User.FindByEmail(ctx, email)
		if err != nil {
			return err
		}

		a := u.Auths().GetByProvider(user.ProviderReearth)
		if a == nil || a.Sub == "" {
			return interfaces.ErrUserInvalidPasswordReset
		}

		pr := user.NewPasswordReset()
		u.SetPasswordReset(pr)

		if err := i.repos.User.Save(ctx, u); err != nil {
			return err
		}

		var TextOut, HTMLOut bytes.Buffer
		link := i.authSrvUIDomain + "/?pwd-reset-token=" + pr.Token
		passwordResetMailContent.UserName = u.Name()
		passwordResetMailContent.ActionURL = htmlTmpl.URL(link)

		if err := authTextTMPL.Execute(&TextOut, passwordResetMailContent); err != nil {
			return err
		}
		if err := authHTMLTMPL.Execute(&HTMLOut, passwordResetMailContent); err != nil {
			return err
		}

		err = i.gateways.Mailer.SendMail(ctx, []mailer.Contact{
			{
				Email: u.Email(),
				Name:  u.Name(),
			},
		}, "Password reset", TextOut.String(), HTMLOut.String())
		if err != nil {
			return err
		}

		return nil
	})
}

func (i *User) PasswordReset(ctx context.Context, password string, token string) error {
	return Run0(ctx, nil, i.repos, Usecase().Transaction(), func(ctx context.Context) error {
		u, err := i.repos.User.FindByPasswordResetRequest(ctx, token)
		if err != nil {
			return err
		}

		passwordReset := u.PasswordReset()
		ok := passwordReset.Validate(token)
		if !ok {
			return interfaces.ErrUserInvalidPasswordReset
		}

		a := u.Auths().GetByProvider(user.ProviderReearth)
		if a == nil || a.Sub == "" {
			return interfaces.ErrUserInvalidPasswordReset
		}

		if err := u.SetPassword(password); err != nil {
			return err
		}

		u.SetPasswordReset(nil)

		if err := i.repos.User.Save(ctx, u); err != nil {
			return err
		}

		return nil
	})
}

type UserQuery struct {
	repos []repo.User
}

func NewUserQuery(primary repo.User, repos ...repo.User) *UserQuery {
	return &UserQuery{
		repos: append([]repo.User{primary}, repos...),
	}
}

func (q *UserQuery) FetchByID(ctx context.Context, ids user.IDList) (user.List, error) {
	us := make(user.List, len(ids))
	for _, r := range q.repos {
		u, err := r.FindByIDs(ctx, ids)
		if err != nil {
			return nil, err
		}

		for i, uu := range u {
			if uu != nil && us[i] == nil {
				us[i] = uu
			}
		}
	}

	return us, nil
}

func (q *UserQuery) FetchBySub(ctx context.Context, sub string) (*user.User, error) {
	for _, r := range q.repos {
		u, err := r.FindBySub(ctx, sub)
		if errors.Is(err, rerror.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		return u, nil
	}
	return nil, rerror.ErrNotFound
}

func (q *UserQuery) FetchByNameOrEmail(ctx context.Context, nameOrEmail string) (*user.Simple, error) {
	for _, r := range q.repos {
		u, err := r.FindByNameOrEmail(ctx, nameOrEmail)
		if errors.Is(err, rerror.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		return user.SimpleFrom(u), nil
	}
	return nil, nil
}

func (q *UserQuery) SearchUser(ctx context.Context, keyword string) (user.SimpleList, error) {
	for _, r := range q.repos {
		u, err := r.SearchByKeyword(ctx, keyword)
		if errors.Is(err, rerror.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if len(u) > 0 {
			return lo.FilterMap(u, func(u *user.User, _ int) (*user.Simple, bool) {
				su := user.SimpleFrom(u)
				if su == nil {
					return nil, false
				}
				return su, true
			}), nil
		}
	}
	return nil, nil
}
