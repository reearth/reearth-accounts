package interfaces

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/usecase"
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
	Name                 *string
	Email                *string
	Lang                 *language.Tag
	Theme                *user.Theme
	Password             *string
	PasswordConfirmation *string
}

type UserQuery interface {
	FetchByID(context.Context, user.IDList) (user.List, error)
	FetchBySub(context.Context, string) (*user.User, error)
	FetchByNameOrEmail(context.Context, string) (*user.Simple, error)
	SearchUser(ctx context.Context, keyword string) (user.List, error)
}

type User interface {
	UserQuery

	// sign up
	Signup(context.Context, SignupParam) (*user.User, error)
	SignupOIDC(context.Context, SignupOIDCParam) (*user.User, error)

	// editing me
	UpdateMe(context.Context, UpdateMeParam, *usecase.Operator) (*user.User, error)
	RemoveMyAuth(context.Context, string, *usecase.Operator) (*user.User, error)
	DeleteMe(context.Context, user.ID, *usecase.Operator) error

	// built-in auth server
	CreateVerification(context.Context, string) error
	VerifyUser(context.Context, string) (*user.User, error)
	StartPasswordReset(context.Context, string) error
	PasswordReset(context.Context, string, string) error
}
