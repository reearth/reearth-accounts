package interfaces

import (
	"context"

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

type UserQuery interface {
	FetchByID(context.Context, user.IDList) (user.List, error)
	FetchByIDsWithPagination(ctx context.Context, ids user.IDList, alias *string, pagination FetchByIDsWithPaginationParam) (FetchByIDsWithPaginationResult, error)
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

	// session management
	Logout(context.Context, *workspace.Operator) (*user.User, error)

	// editing me
	DeleteMe(context.Context, user.ID, *workspace.Operator) error
	RemoveMyAuth(context.Context, string, *workspace.Operator) (*user.User, error)
	UpdateMe(context.Context, UpdateMeParam, *workspace.Operator) (*user.User, error)

	// built-in auth server
	CreateVerification(context.Context, string) error
	VerifyUser(context.Context, string) (*user.User, error)
	StartPasswordReset(context.Context, string) error
	PasswordReset(context.Context, string, string) error
}
