package interfaces

import (
	"context"

	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/rerror"
	"golang.org/x/text/language"
)

var (
	ErrUserInvalidPasswordConfirmation = rerror.NewE(i18n.T("invalid password confirmation"))
	ErrUserInvalidPasswordReset        = rerror.NewE(i18n.T("invalid password reset request"))
	ErrUserInvalidLang                 = rerror.NewE(i18n.T("invalid lang"))
	ErrSignupInvalidSecret             = rerror.NewE(i18n.T("invalid secret"))
	ErrInvalidUserEmail                = rerror.NewE(i18n.T("invalid email"))
	ErrNotVerifiedUser                 = rerror.NewE(i18n.T("not verified user"))
	ErrInvalidEmailOrPassword          = rerror.NewE(i18n.T("invalid email or password"))
	ErrInvalidCurrentPassword          = rerror.NewE(i18n.T("current password is required or incorrect"))
	ErrUserAlreadyExists               = rerror.NewE(i18n.T("user already exists"))
	ErrUserAliasAlreadyExists          = rerror.NewE(i18n.T("user alias already exists"))
	ErrWorkspaceAliasAlreadyExists     = rerror.NewE(i18n.T("workspace alias already exists"))
)

type SignupOIDCParam struct {
	AccessToken string
	Issuer      string
	Sub         string
	Email       string
	Name        string
	Secret      *string
	User        SignupUserParam
}

type SyncSSOUserParam struct {
	Email       string
	Lang        *language.Tag
	Name        string
	Sub         string // samlp|<organization-id>|<idpID>
	Theme       *user.Theme
	UserID      *user.ID
	WorkspaceID *workspace.ID
}

type SignupUserParam struct {
	UserID      *user.ID
	Lang        *language.Tag
	Theme       *user.Theme
	WorkspaceID *workspace.ID
}

type SignupParam struct {
	Email       string
	Name        string
	Password    string
	Secret      *string
	Lang        *language.Tag
	Theme       *user.Theme
	UserID      *user.ID
	WorkspaceID *workspace.ID
	MockAuth    bool
}

type UserFindOrCreateParam struct {
	Sub   string
	ISS   string
	Token string
}

type GetUserByCredentials struct {
	Email    string
	Password string
}

type UpdateMeParam struct {
	Alias                *string
	Description          *string
	Email                *string
	Lang                 *language.Tag
	Name                 *string
	Password             *string
	PasswordConfirmation *string
	PhotoURL             *string
	Theme                *user.Theme
	Website              *string
}

type FetchByIDsWithPaginationParam struct {
	Page int64
	Size int64
}

type FetchByIDsWithPaginationResult struct {
	Users      user.List
	TotalCount int
}

type FindAllUsersParam struct {
	Keyword *string
	Status  user.StatusFilter
	Page    int64
	Size    int64
	// Operator identifies the caller for the maintainer-role permission check in
	// (*User).FindAll. Required; requests without an operator are rejected.
	Operator *workspace.Operator
}

type FindAllUsersResult struct {
	Users      user.List
	TotalCount int
}

type UserQuery interface {
	FetchByID(context.Context, user.IDList) (user.List, error)
	FetchByIDsWithPagination(ctx context.Context, ids user.IDList, alias *string, pagination FetchByIDsWithPaginationParam) (FetchByIDsWithPaginationResult, error)
	// FindAll lists users across all tenants, unfiltered by any per-workspace
	// scoping. Restricted to the maintainer role (see (*User).FindAll /
	// checkMaintainerPermission) since it exposes every user across every tenant.
	FindAll(context.Context, FindAllUsersParam) (FindAllUsersResult, error)
	FetchBySub(context.Context, string) (*user.User, error)
	FetchByNameOrAlias(context.Context, string) (user.List, error)
	FetchByNameOrEmail(context.Context, string) (*user.Simple, error)
	FetchByAlias(context.Context, string) (*user.User, error)
	SearchUser(ctx context.Context, keyword string) (user.List, error)
}

type User interface {
	UserQuery

	// sign up
	Signup(context.Context, SignupParam) (*user.User, error)
	SignupOIDC(context.Context, SignupOIDCParam) (*user.User, error)
	SyncSSOUser(context.Context, SyncSSOUserParam) (*user.User, error)

	// session management
	Logout(context.Context, *workspace.Operator) (*user.User, error)

	// editing me
	DeleteMe(context.Context, user.ID, *workspace.Operator) error
	RemoveMyAuth(context.Context, string, *workspace.Operator) (*user.User, error)
	UpdateMe(context.Context, UpdateMeParam, *workspace.Operator) (*user.User, error)

	// admin: deactivate soft-deletes a user (sets deleted_at); restore reverses it.
	// Same permission model as workspace's Deactivate/Restore (Cerbos, falling back
	// to a maintainer-role check).
	Deactivate(ctx context.Context, id user.ID, operator *workspace.Operator) (*user.User, error)
	Restore(ctx context.Context, id user.ID, operator *workspace.Operator) (*user.User, error)

	// User mutations by Firebase sub, gated on the caller holding the maintainer
	// role (JWT required; see checkMaintainerPermission).
	UpdateUserBySub(ctx context.Context, sub string, name *string, operator *workspace.Operator) error
	SetPlatformRolesBySub(ctx context.Context, sub string, roleNames []string, operator *workspace.Operator) error

	// built-in auth server
	CreateVerification(context.Context, string) error
	VerifyUser(context.Context, string) (*user.User, error)
	StartPasswordReset(context.Context, string) error
	PasswordReset(context.Context, string, string) error

	// mfa
	DisableMFA(context.Context, *workspace.Operator) error
	EnableMFA(context.Context, *workspace.Operator) (enrollmentURL string, err error)
	GetMFAStatus(context.Context, *workspace.Operator) (gateway.MFAStatus, error)
	// RegenerateMFARecoveryCode mints a fresh MFA recovery code, invalidating
	// the previous one. currentPassword is required and verified when the
	// account has a "reearth" (password) auth record, since this credential
	// can bypass MFA on future logins; ignored for SSO-only accounts.
	RegenerateMFARecoveryCode(ctx context.Context, operator *workspace.Operator, currentPassword string) (recoveryCode string, err error)
}
