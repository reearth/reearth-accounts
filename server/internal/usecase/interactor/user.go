package interactor

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	htmlTmpl "html/template"
	"time"

	"github.com/reearth/reearth-accounts/server/internal/rbac"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/pagination"
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/log"
	"github.com/reearth/reearthx/mailer"
	"github.com/reearth/reearthx/rerror"
)

type User struct {
	repos           *repo.Container
	gateways        *gateway.Container
	cerbos          interfaces.Cerbos
	signupSecret    string
	authSrvUIDomain string
	allowedISS      []string
	query           interfaces.UserQuery
}

var (
	passwordResetMailContent = mailContent{
		Message:     "Thank you for using Re:Earth. We've received a request to reset your password. If this was you, please click the link below to confirm and change your password.",
		Suffix:      "If you did not mean to reset your password, then you can ignore this email.",
		ActionLabel: "Confirm to reset your password",
	}
)

func NewUser(r *repo.Container, g *gateway.Container, cerbos interfaces.Cerbos, signupSecret, authSrcUIDomain string, allowedISS ...string) interfaces.User {
	var repos []user.Repo
	if r != nil {
		repos = []user.Repo{r.User}
	}
	return &User{
		repos:           r,
		gateways:        g,
		cerbos:          cerbos,
		signupSecret:    signupSecret,
		authSrvUIDomain: authSrcUIDomain,
		allowedISS:      allowedISS,
		query: &UserQuery{
			repos: repos,
		},
	}
}

func NewMultiUser(r *repo.Container, g *gateway.Container, cerbos interfaces.Cerbos, signupSecret, authSrcUIDomain string, users []user.Repo, allowedISS ...string) interfaces.User {
	return &User{
		repos:           r,
		gateways:        g,
		cerbos:          cerbos,
		signupSecret:    signupSecret,
		authSrvUIDomain: authSrcUIDomain,
		allowedISS:      allowedISS,
		query: &UserQuery{
			repos: append([]user.Repo{r.User}, users...),
		},
	}
}

func (i *User) FetchByID(ctx context.Context, ids user.IDList) (user.List, error) {
	return i.query.FetchByID(ctx, ids)
}

func (i *User) FetchByIDsWithPagination(ctx context.Context, ids user.IDList, alias *string, pagination interfaces.FetchByIDsWithPaginationParam) (interfaces.FetchByIDsWithPaginationResult, error) {
	return i.query.FetchByIDsWithPagination(ctx, ids, alias, pagination)
}

// FindAll lists users across all tenants. Restricted to the owner
// role (see checkOwnerPermission) since it exposes every user across every tenant.
func (i *User) FindAll(ctx context.Context, param interfaces.FindAllUsersParam) (interfaces.FindAllUsersResult, error) {
	if param.Operator == nil || param.Operator.User == nil {
		return interfaces.FindAllUsersResult{}, interfaces.ErrInvalidOperator
	}
	if err := i.checkMaintainerPermission(ctx, param.Operator, rbac.ActionList); err != nil {
		return interfaces.FindAllUsersResult{}, err
	}
	return i.query.FindAll(ctx, param)
}

func (i *User) FetchBySub(ctx context.Context, sub string) (*user.User, error) {
	return i.query.FetchBySub(ctx, sub)
}

func (i *User) FetchByNameOrEmail(ctx context.Context, nameOrEmail string) (*user.Simple, error) {
	return i.query.FetchByNameOrEmail(ctx, nameOrEmail)
}

func (i *User) SearchUser(ctx context.Context, keyword string) (user.List, error) {
	return i.query.SearchUser(ctx, keyword)
}

func (i *User) FetchByAlias(ctx context.Context, alias string) (*user.User, error) {
	return i.query.FetchByAlias(ctx, alias)
}

func (i *User) FetchByNameOrAlias(ctx context.Context, nameOrAlias string) (user.List, error) {
	return i.query.FetchByNameOrAlias(ctx, nameOrAlias)
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

func (i *User) Logout(ctx context.Context, operator *workspace.Operator) (*user.User, error) {
	if operator.User == nil {
		return nil, interfaces.ErrInvalidOperator
	}

	return Run1(ctx, operator, i.repos, Usecase().Transaction(), func(ctx context.Context) (*user.User, error) {
		u, err := i.repos.User.FindByID(ctx, *operator.User)
		if err != nil {
			return nil, err
		}

		u.SetLatestLogoutAt(time.Now())

		if err := i.repos.User.Save(ctx, u); err != nil {
			return nil, err
		}

		return u, nil
	})
}

func (i *User) UpdateMe(ctx context.Context, p interfaces.UpdateMeParam, operator *workspace.Operator) (u *user.User, err error) {
	if operator.User == nil {
		return nil, interfaces.ErrInvalidOperator
	}

	return Run1(ctx, operator, i.repos, Usecase().Transaction(), func(ctx context.Context) (*user.User, error) {
		if p.Password != nil {
			if p.PasswordConfirmation == nil || *p.Password != *p.PasswordConfirmation {
				return nil, interfaces.ErrUserInvalidPasswordConfirmation
			}
		}

		var ws *workspace.Workspace

		u, err = i.repos.User.FindByID(ctx, *operator.User)
		if err != nil {
			return nil, err
		}

		ws, err = i.repos.Workspace.FindByID(ctx, u.Workspace())
		if err != nil && !errors.Is(err, rerror.ErrNotFound) {
			return nil, err
		}

		if ws == nil {
			return nil, rerror.ErrNotFound
		}

		if p.Alias != nil && *p.Alias != u.Alias() {
			existingUser, err := i.repos.User.FindByAlias(ctx, *p.Alias)
			if err != nil && !errors.Is(err, rerror.ErrNotFound) {
				return nil, err
			}
			if existingUser != nil && existingUser.ID() != u.ID() {
				return nil, interfaces.ErrUserAliasAlreadyExists
			}
			u.UpdateAlias(*p.Alias)
			ws.UpdateAlias(*p.Alias)
		}
		if p.Name != nil && *p.Name != u.Name() {
			oldName := u.Name()
			u.UpdateName(*p.Name)

			tn := ws.Name()
			if tn == "" || tn == oldName {
				ws.Rename(*p.Name)
			}
		}
		if p.Email != nil {
			if err = u.UpdateEmail(*p.Email); err != nil {
				return nil, err
			}
		}

		if u.Metadata() != nil {
			if p.Lang != nil {
				u.Metadata().LangFrom(p.Lang.String())
			}

			if p.Theme != nil {
				u.Metadata().SetTheme(*p.Theme)
			}

			if p.Description != nil {
				u.Metadata().SetDescription(*p.Description)
			}

			if p.Website != nil {
				u.Metadata().SetWebsite(*p.Website)
			}

			if p.PhotoURL != nil {
				u.Metadata().SetPhotoURL(*p.PhotoURL)
			}
		}

		// SEC-03: this sets a new password from the session alone, with no
		// current-password or step-up MFA check. Re-authentication for
		// sensitive account mutations is planned to be covered by MFA
		// confirmation in a future change, not now.
		if p.Password != nil && u.HasAuthProvider("reearth") {
			if err := u.SetPassword(*p.Password); err != nil {
				return nil, err
			}
		}

		// Sync external IdP users to their provider, routed per auth record so
		// Auth0 subs go to Auth0. CIP (Cloud Identity Platform, used by Veda) is
		// deliberately skipped: the accounts DB record is the source of truth for
		// display name there, and Veda manages its own IdP state independently.
		if p.Name != nil || p.Email != nil || p.Password != nil {
			for _, a := range u.Auths() {
				if gateway.Provider(a.Provider) == gateway.ProviderCIP || a.Provider == "" {
					continue
				}
				authenticator := i.gateways.AuthenticatorFor(a.Provider)
				if authenticator == nil {
					continue
				}
				if _, err = authenticator.UpdateUser(ctx, gateway.AuthenticatorUpdateUserParam{
					ID:       a.Sub,
					Name:     p.Name,
					Email:    p.Email,
					Password: p.Password,
				}); err != nil {
					return nil, err
				}
			}
		}

		// Update personal workspace metadata fields (description, website, photoURL)
		if p.Description != nil || p.Website != nil || p.PhotoURL != nil {
			if ws != nil && ws.IsPersonal() {
				metadata := ws.Metadata()
				if p.Description != nil {
					metadata.SetDescription(*p.Description)
				}
				if p.Website != nil {
					metadata.SetWebsite(*p.Website)
				}
				if p.PhotoURL != nil {
					metadata.SetPhotoURL(*p.PhotoURL)
				}
				ws.SetMetadata(*metadata)
			}
		}

		if ws != nil {
			err = i.repos.Workspace.Save(ctx, ws)
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

func (i *User) RemoveMyAuth(ctx context.Context, authProvider string, operator *workspace.Operator) (u *user.User, err error) {
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

// SEC-03: disabling MFA here only requires a valid operator/session, with no
// password confirmation, current-MFA challenge, or recent-auth check guarding
// this privilege-lowering operation. Re-authentication for sensitive account
// mutations like this is planned to be covered by MFA confirmation in a
// future change, not now.
func (i *User) DisableMFA(ctx context.Context, operator *workspace.Operator) error {
	if operator == nil || operator.User == nil {
		return interfaces.ErrInvalidOperator
	}
	return Run0(ctx, operator, i.repos, Usecase(), func(ctx context.Context) error {
		u, err := i.repos.User.FindByID(ctx, *operator.User)
		if err != nil {
			return err
		}
		a := u.Auths().GetByProvider(user.ProviderAuth0)
		if a == nil {
			return rerror.NewE(i18n.T("no authenticator found"))
		}
		authenticator := i.gateways.AuthenticatorFor(a.Provider)
		if authenticator == nil {
			return rerror.NewE(i18n.T("no authenticator found"))
		}
		return authenticator.DisableMFA(ctx, a.Sub)
	})
}

func (i *User) EnableMFA(ctx context.Context, operator *workspace.Operator) (string, error) {
	if operator == nil || operator.User == nil {
		return "", interfaces.ErrInvalidOperator
	}
	return Run1(ctx, operator, i.repos, Usecase(), func(ctx context.Context) (string, error) {
		u, err := i.repos.User.FindByID(ctx, *operator.User)
		if err != nil {
			return "", err
		}
		a := u.Auths().GetByProvider(user.ProviderAuth0)
		if a == nil {
			return "", rerror.NewE(i18n.T("no authenticator found"))
		}
		authenticator := i.gateways.AuthenticatorFor(a.Provider)
		if authenticator == nil {
			return "", rerror.NewE(i18n.T("no authenticator found"))
		}
		return authenticator.EnableMFA(ctx, a.Sub)
	})
}

func (i *User) GetMFAStatus(ctx context.Context, operator *workspace.Operator) (gateway.MFAStatus, error) {
	if operator == nil || operator.User == nil {
		return gateway.MFAStatus{}, interfaces.ErrInvalidOperator
	}
	return Run1(ctx, operator, i.repos, Usecase(), func(ctx context.Context) (gateway.MFAStatus, error) {
		u, err := i.repos.User.FindByID(ctx, *operator.User)
		if err != nil {
			return gateway.MFAStatus{}, err
		}
		a := u.Auths().GetByProvider(user.ProviderAuth0)
		if a == nil {
			return gateway.MFAStatus{Enrolled: false}, nil
		}
		authenticator := i.gateways.AuthenticatorFor(a.Provider)
		if authenticator == nil {
			return gateway.MFAStatus{Enrolled: false}, nil
		}
		return authenticator.GetMFAStatus(ctx, a.Sub)
	})
}

func (i *User) RegenerateMFARecoveryCode(ctx context.Context, operator *workspace.Operator, currentPassword string) (string, error) {
	if operator == nil || operator.User == nil {
		return "", interfaces.ErrInvalidOperator
	}
	return Run1(ctx, operator, i.repos, Usecase().Transaction(), func(ctx context.Context) (string, error) {
		u, err := i.repos.User.FindByID(ctx, *operator.User)
		if err != nil {
			return "", err
		}

		// This mints a credential that can bypass MFA on future logins, so
		// require and verify the current password first for accounts that have
		// one; SSO-only accounts have no local password to check.
		if u.HasAuthProvider("reearth") {
			matched, mErr := u.MatchPassword(currentPassword)
			if mErr != nil || !matched {
				return "", interfaces.ErrInvalidCurrentPassword
			}
		}

		a := u.Auths().GetByProvider(user.ProviderAuth0)
		if a == nil {
			return "", rerror.NewE(i18n.T("no authenticator found"))
		}
		authenticator := i.gateways.AuthenticatorFor(a.Provider)
		if authenticator == nil {
			return "", rerror.NewE(i18n.T("no authenticator found"))
		}
		return authenticator.RegenerateMFARecoveryCode(ctx, a.Sub)
	})
}

func (i *User) DeleteMe(ctx context.Context, userID user.ID, operator *workspace.Operator) (err error) {
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

		for _, ws := range workspaces {
			if !ws.IsPersonal() && !ws.Members().IsOnlyOwner(u.ID()) {
				if err := ws.Members().Leave(u.ID()); err != nil {
					if errors.Is(err, workspace.ErrTargetUserNotInTheWorkspace) {
						// User already removed by a concurrent change; nothing to save
						continue
					}
					return err
				}
				updatedWorkspaces = append(updatedWorkspaces, ws)
				continue
			}

			deletedWorkspaces = append(deletedWorkspaces, ws.ID())
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

// Deactivate soft-deletes a user (sets deleted_at). Same permission model as
// workspace's Deactivate: Cerbos, falling back to a maintainer-role check.
func (i *User) Deactivate(ctx context.Context, id user.ID, operator *workspace.Operator) (*user.User, error) {
	if operator.User == nil {
		return nil, interfaces.ErrInvalidOperator
	}

	return Run1(ctx, operator, i.repos, Usecase().Transaction(), func(ctx context.Context) (*user.User, error) {
		u, err := i.repos.User.FindByID(ctx, id)
		if err != nil {
			return nil, err
		}

		if err := i.checkMaintainerPermission(ctx, operator, rbac.ActionEdit); err != nil {
			return nil, err
		}

		u.Deactivate()

		if err := i.repos.User.Save(ctx, u); err != nil {
			return nil, err
		}

		return u, nil
	})
}

// Restore reverses Deactivate (clears deleted_at). Same permission model as
// Deactivate.
func (i *User) Restore(ctx context.Context, id user.ID, operator *workspace.Operator) (*user.User, error) {
	if operator.User == nil {
		return nil, interfaces.ErrInvalidOperator
	}

	return Run1(ctx, operator, i.repos, Usecase().Transaction(), func(ctx context.Context) (*user.User, error) {
		u, err := i.repos.User.FindByID(ctx, id)
		if err != nil {
			return nil, err
		}

		if err := i.checkMaintainerPermission(ctx, operator, rbac.ActionEdit); err != nil {
			return nil, err
		}

		u.Reactivate()

		if err := i.repos.User.Save(ctx, u); err != nil {
			return nil, err
		}

		return u, nil
	})
}

// checkMaintainerPermission gates admin-only user actions (Deactivate/Restore,
// FindAll, by-sub mutations) to principals holding the elevated "maintainer" or
// "owner" global role, either via Cerbos or, when Cerbos isn't configured (e.g.
// local/mock-auth dev), by re-checking the operator's own Permittable directly.
// "owner" here is a global Permittable role (LINKS-Veda's admin account), not a
// per-workspace role. Mirrors Permittable.checkManageRolesPermission.
func (i *User) checkMaintainerPermission(ctx context.Context, operator *workspace.Operator, action string) error {
	if i.cerbos != nil {
		result, err := i.cerbos.CheckPermission(ctx, *operator.User, interfaces.CheckPermissionParam{
			Service:  rbac.ServiceName,
			Resource: rbac.ResourceUser,
			Action:   action,
		})
		if err != nil {
			return err
		}
		if result != nil {
			if !result.Allowed {
				return interfaces.ErrPermissionDenied
			}
			return nil
		}
	}

	p, err := i.repos.Permittable.FindByUserID(ctx, *operator.User)
	if err != nil && !errors.Is(err, rerror.ErrNotFound) {
		return err
	}
	if p == nil {
		return interfaces.ErrPermissionDenied
	}

	roles, err := i.repos.Role.FindByIDs(ctx, p.RoleIDs())
	if err != nil {
		return err
	}
	for _, r := range roles {
		if r.Name() == role.RoleMaintainer.String() || r.Name() == role.RoleOwner.String() {
			return nil
		}
	}

	return interfaces.ErrPermissionDenied
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
	var contact mailer.Contact
	var mailText, mailHTML string

	if err := Run0(ctx, nil, i.repos, Usecase().Transaction(), func(ctx context.Context) error {
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

		if err = i.repos.User.Save(ctx, u); err != nil {
			return err
		}

		var TextOut, HTMLOut bytes.Buffer
		link := i.authSrvUIDomain + "/?pwd-reset-token=" + pr.Token
		content := mailContent{
			UserName:    u.Name(),
			ActionURL:   htmlTmpl.URL(link),
			Message:     passwordResetMailContent.Message,
			Suffix:      passwordResetMailContent.Suffix,
			ActionLabel: passwordResetMailContent.ActionLabel,
		}

		if err = authTextTMPL.Execute(&TextOut, content); err != nil {
			return err
		}
		if err = authHTMLTMPL.Execute(&HTMLOut, content); err != nil {
			return err
		}

		contact = mailer.Contact{Email: u.Email(), Name: u.Name()}
		mailText = TextOut.String()
		mailHTML = HTMLOut.String()

		return nil
	}); err != nil {
		return err
	}

	return i.gateways.Mailer.SendMail(ctx, []mailer.Contact{contact}, "Password reset", mailText, mailHTML)
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
	repos []user.Repo
}

func NewUserQuery(primary user.Repo, repos ...user.Repo) *UserQuery {
	return &UserQuery{
		repos: append([]user.Repo{primary}, repos...),
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

func (q *UserQuery) FetchByIDsWithPagination(ctx context.Context, ids user.IDList, alias *string, paginationParam interfaces.FetchByIDsWithPaginationParam) (interfaces.FetchByIDsWithPaginationResult, error) {
	users, pageInfo, err := q.repos[0].FindByIDsWithPagination(ctx, ids, alias, pagination.ToPagination(paginationParam.Page, paginationParam.Size))
	if err != nil {
		return interfaces.FetchByIDsWithPaginationResult{}, err
	}

	return interfaces.FetchByIDsWithPaginationResult{
		Users:      user.List(users),
		TotalCount: int(pageInfo.TotalCount),
	}, nil
}

func (q *UserQuery) FindAll(ctx context.Context, param interfaces.FindAllUsersParam) (interfaces.FindAllUsersResult, error) {
	status := param.Status
	if status == "" {
		status = user.StatusActive
	}
	users, pageInfo, err := q.repos[0].FindAllWithPagination(ctx, param.Keyword, status, pagination.ToPagination(param.Page, param.Size))
	if err != nil {
		return interfaces.FindAllUsersResult{}, err
	}

	return interfaces.FindAllUsersResult{
		Users:      user.List(users),
		TotalCount: int(pageInfo.TotalCount),
	}, nil
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
	return nil, rerror.ErrNotFound
}

func (q *UserQuery) SearchUser(ctx context.Context, keyword string) (user.List, error) {
	for _, r := range q.repos {
		u, err := r.SearchByKeyword(ctx, keyword)
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

func (q *UserQuery) FetchByAlias(ctx context.Context, alias string) (*user.User, error) {
	for _, r := range q.repos {
		u, err := r.FindByAlias(ctx, alias)
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

func (q *UserQuery) FetchByNameOrAlias(ctx context.Context, nameOrAlias string) (user.List, error) {
	for _, r := range q.repos {
		u, err := r.FindByNameOrAlias(ctx, nameOrAlias)
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

// UpdateUserBySub updates a user's mutable fields (currently name) looked up by Firebase sub.
// The caller must hold the maintainer role (see checkMaintainerPermission).
// The personal workspace is renamed in sync when its name still matches the old user name.
func (i *User) UpdateUserBySub(ctx context.Context, sub string, name *string, operator *workspace.Operator) error {
	if operator == nil || operator.User == nil {
		return interfaces.ErrInvalidOperator
	}
	if err := i.checkMaintainerPermission(ctx, operator, rbac.ActionEdit); err != nil {
		return err
	}
	if name == nil {
		return nil
	}

	u, err := i.repos.User.FindBySub(ctx, sub)
	if err != nil {
		return err
	}

	oldName := u.Name()
	u.UpdateName(*name)

	if err := i.repos.User.Save(ctx, u); err != nil {
		return err
	}

	// Keep the personal workspace name in sync when it still matched the old user name.
	ws, err := i.repos.Workspace.FindByID(ctx, u.Workspace())
	if err != nil && !errors.Is(err, rerror.ErrNotFound) {
		return err
	}
	if ws != nil && ws.IsPersonal() {
		tn := ws.Name()
		if tn == "" || tn == oldName {
			ws.Rename(*name)
			if err := i.repos.Workspace.Save(ctx, ws); err != nil {
				log.Warnfc(ctx, "UpdateUserBySub: workspace rename failed (non-fatal): %v", err)
			}
		}
	}

	return nil
}

// SetPlatformRolesBySub replaces a user's global platform roles, looked up by Firebase sub.
// The caller must hold the maintainer role (see checkMaintainerPermission). An empty
// roleNames slice clears all platform roles.
func (i *User) SetPlatformRolesBySub(ctx context.Context, sub string, roleNames []string, operator *workspace.Operator) error {
	if operator == nil || operator.User == nil {
		return interfaces.ErrInvalidOperator
	}
	if err := i.checkMaintainerPermission(ctx, operator, rbac.ActionEdit); err != nil {
		return err
	}

	u, err := i.repos.User.FindBySub(ctx, sub)
	if err != nil {
		return err
	}

	// Resolve role names → IDs
	rids := make(id.RoleIDList, 0, len(roleNames))
	for _, name := range roleNames {
		r, err := i.repos.Role.FindByName(ctx, name)
		if err != nil {
			return err
		}
		rids = append(rids, r.ID())
	}

	// Find or create the permittable record for this user
	p, err := i.repos.Permittable.FindByUserID(ctx, u.ID())
	if err != nil && !errors.Is(err, rerror.ErrNotFound) {
		return err
	}
	if p == nil {
		p, err = permittable.New().NewID().UserID(u.ID()).Build()
		if err != nil {
			return err
		}
	}

	p.EditRoleIDs(rids)
	return i.repos.Permittable.Save(ctx, *p)
}
