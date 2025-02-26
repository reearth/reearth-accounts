package gql

import (
	"context"

	"github.com/eukarya-inc/reearth-accounts/internal/adapter/gql/gqlmodel"
	"golang.org/x/text/language"

	"github.com/reearth/reearthx/account/accountdomain"
	"github.com/reearth/reearthx/account/accountusecase/accountinterfaces"
)

func (r *mutationResolver) UpdateMe(ctx context.Context, input gqlmodel.UpdateMeInput) (*gqlmodel.UpdateMePayload, error) {
	var lang language.Tag
	if input.Lang != nil {
		lang = language.Make(*input.Lang)
	}
	res, err := usecases(ctx).User.UpdateMe(ctx, accountinterfaces.UpdateMeParam{
		Name:                 input.Name,
		Email:                input.Email,
		Lang:                 &lang,
		Theme:                gqlmodel.ToTheme(input.Theme),
		Password:             input.Password,
		PasswordConfirmation: input.PasswordConfirmation,
	}, getOperator(ctx))
	if err != nil {
		return nil, err
	}

	return &gqlmodel.UpdateMePayload{Me: gqlmodel.ToMe(res)}, nil
}

func (r *mutationResolver) RemoveMyAuth(ctx context.Context, input gqlmodel.RemoveMyAuthInput) (*gqlmodel.UpdateMePayload, error) {
	res, err := usecases(ctx).User.RemoveMyAuth(ctx, input.Auth, getOperator(ctx))
	if err != nil {
		return nil, err
	}

	return &gqlmodel.UpdateMePayload{Me: gqlmodel.ToMe(res)}, nil
}

func (r *mutationResolver) DeleteMe(ctx context.Context, input gqlmodel.DeleteMeInput) (*gqlmodel.DeleteMePayload, error) {
	uid, err := gqlmodel.ToID[accountdomain.User](input.UserID)
	if err != nil {
		return nil, err
	}

	if err := usecases(ctx).User.DeleteMe(ctx, uid, getOperator(ctx)); err != nil {
		return nil, err
	}

	return &gqlmodel.DeleteMePayload{UserID: input.UserID}, nil
}
