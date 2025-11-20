//go:generate go run github.com/Khan/genqlient

package proxy

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/util"

	_ "github.com/Khan/genqlient/generate"
	"github.com/Khan/genqlient/graphql"
	"github.com/reearth/reearth-accounts/server/internal/usecase"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
)

type User struct {
	http     HTTPClient
	gql      graphql.Client
	endpoint string
}

func NewUser(endpoint string, h HTTPClient) interfaces.User {
	return &User{
		http:     h,
		endpoint: endpoint,
		gql:      graphql.NewClient(endpoint, h),
	}
}

func (u *User) FetchBySub(ctx context.Context, sub string) (*user.User, error) {
	panic("unsupported")
}

func (u *User) FetchByID(ctx context.Context, ids user.IDList) (user.List, error) {
	return UserByIDsResponseTo(UserByIDs(ctx, u.gql, ids.Strings()))
}

func (u *User) Signup(ctx context.Context, param interfaces.SignupParam) (*user.User, error) {
	input := SignupInput{
		Id:          param.UserID.String(),
		WorkspaceID: param.WorkspaceID.String(),
		Name:        param.Name,
		Email:       param.Email,
		Password:    param.Password,
		Secret:      *param.Secret,
		Lang:        param.Lang.String(),
		Theme:       string(*param.Theme),
	}
	res, err := Signup(ctx, u.gql, input)
	if err != nil {
		return nil, err
	}
	return FragmentToUser(res.Signup.User.FragmentUser)
}

func (u *User) SignupOIDC(ctx context.Context, param interfaces.SignupOIDCParam) (*user.User, error) {
	input := SignupOIDCInput{
		Id:          param.User.UserID.String(),
		Name:        param.Name,
		Email:       param.Email,
		Sub:         param.Sub,
		Lang:        param.User.Lang.String(),
		WorkspaceId: param.User.WorkspaceID.String(),
		Secret:      *param.Secret,
	}
	res, err := SignupOIDC(ctx, u.gql, input)
	if err != nil {
		return nil, err
	}
	return FragmentToUser(res.SignupOIDC.User.FragmentUser)
}

func (u *User) FindOrCreate(ctx context.Context, param interfaces.UserFindOrCreateParam) (*user.User, error) {
	input := FindOrCreateInput{
		Sub:   param.Sub,
		Iss:   param.ISS,
		Token: param.Token,
	}
	res, err := FindOrCreate(ctx, u.gql, input)
	if err != nil {
		return nil, err
	}
	return FragmentToUser(res.FindOrCreate.User.FragmentUser)
}

func (u *User) UpdateMe(ctx context.Context, param interfaces.UpdateMeParam, op *usecase.Operator) (*user.User, error) {
	input := UpdateMeInput{
		Name:                 *param.Name,
		Email:                *param.Email,
		Lang:                 param.Lang.String(),
		Theme:                string(*param.Theme),
		Password:             *param.Password,
		PasswordConfirmation: *param.PasswordConfirmation,
	}
	res, err := UpdateMe(ctx, u.gql, input)
	if err != nil {
		return nil, err
	}
	return MeToUser(res.UpdateMe.Me.FragmentMe)
}

func (u *User) RemoveMyAuth(ctx context.Context, auth string, op *usecase.Operator) (*user.User, error) {
	res, err := RemoveMyAuth(ctx, u.gql, RemoveMyAuthInput{Auth: auth})
	if err != nil {
		return nil, err
	}
	return MeToUser(res.RemoveMyAuth.Me.FragmentMe)
}

func (u *User) FetchByNameOrEmail(ctx context.Context, nameOrEmail string) (*user.Simple, error) {
	res, err := UserByNameOrEmail(ctx, u.gql, nameOrEmail)
	if err != nil {
		return nil, err
	}
	r, err := FragmentToUser(res.UserByNameOrEmail.FragmentUser)
	if err != nil {
		return nil, err
	}
	return user.SimpleFrom(r), nil
}

func (u *User) SearchUser(ctx context.Context, keyword string) (user.List, error) {
	res, err := SearchUser(ctx, u.gql, keyword)
	if err != nil {
		return nil, err
	}
	r, err := util.TryMap(res.SearchUser.Users, func(u SearchUserSearchUserSearchUserOutputUsersUser) (*user.User, error) {
		return FragmentToUser(u.FragmentUser)
	})
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (u *User) DeleteMe(ctx context.Context, id user.ID, op *usecase.Operator) error {
	_, err := DeleteMe(ctx, u.gql, DeleteMeInput{UserId: id.String()})
	if err != nil {
		return err
	}
	return nil
}

func (u *User) CreateVerification(ctx context.Context, email string) error {
	_, err := CreateVerification(ctx, u.gql, CreateVerificationInput{Email: email})
	if err != nil {
		return err
	}
	return nil
}

func (u *User) VerifyUser(ctx context.Context, code string) (*user.User, error) {
	res, err := VerifyUser(ctx, u.gql, VerifyUserInput{Code: code})
	if err != nil {
		return nil, err
	}
	return FragmentToUser(res.VerifyUser.User.FragmentUser)

}

func (u *User) StartPasswordReset(ctx context.Context, email string) error {
	_, err := StartPasswordReset(ctx, u.gql, StartPasswordResetInput{Email: email})
	if err != nil {
		return err
	}
	return nil
}

func (u *User) PasswordReset(ctx context.Context, password string, token string) error {
	_, err := PasswordReset(ctx, u.gql, PasswordResetInput{Password: password, Token: token})
	if err != nil {
		return err
	}
	return nil
}
