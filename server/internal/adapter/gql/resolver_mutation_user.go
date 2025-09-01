package gql

import (
	"context"

	"github.com/reearth/reearth-accounts/internal/adapter"
	"github.com/reearth/reearth-accounts/internal/adapter/gql/gqlmodel"
	"github.com/reearth/reearth-accounts/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/pkg/id"
	"golang.org/x/text/language"
)

func (r *mutationResolver) UpdateMe(ctx context.Context, input gqlmodel.UpdateMeInput) (*gqlmodel.UpdateMePayload, error) {
	var lang language.Tag
	if input.Lang != nil {
		lang = language.Make(*input.Lang)
	}
	res, err := usecases(ctx).User.UpdateMe(ctx, interfaces.UpdateMeParam{
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
	uid, err := gqlmodel.ToID[id.User](input.UserID)
	if err != nil {
		return nil, err
	}

	if err := usecases(ctx).User.DeleteMe(ctx, uid, getOperator(ctx)); err != nil {
		return nil, err
	}

	return &gqlmodel.DeleteMePayload{UserID: input.UserID}, nil
}

// Temporary stub implementation to satisfy gqlgen after migrating GraphQL files from reearthx/account.
// This resolver was added to avoid compile-time errors.
// Will be implemented if needed, or removed if unused after migration.
func (r *mutationResolver) SignUp(ctx context.Context, input gqlmodel.SignUpInput) (*gqlmodel.UserPayload, error) {
	return nil, nil
}

func (r *mutationResolver) SignupOidc(ctx context.Context, input gqlmodel.SignupOIDCInput) (*gqlmodel.UserPayload, error) {
	au := adapter.GetAuthInfo(ctx)
	if au == nil {
		return nil, interfaces.ErrOperationDenied
	}

	var lang language.Tag
	if input.Lang != nil {
		lang = language.Make(*input.Lang)
	}
	u, err := usecases(ctx).User.SignupOIDC(ctx, interfaces.SignupOIDCParam{
		Sub:         au.Sub,
		AccessToken: au.Token,
		Issuer:      au.Iss,
		Email:       au.Email,
		Name:        au.Name,
		Secret:      input.Secret,
		User: interfaces.SignupUserParam{
			Lang:        &lang,
			UserID:      gqlmodel.ToIDRef[id.User](input.ID),
			WorkspaceID: gqlmodel.ToIDRef[id.Workspace](input.WorkspaceID),
		},
	})
	if err != nil {
		return nil, err
	}

	return &gqlmodel.UserPayload{User: gqlmodel.ToUser(u)}, nil
}

// Temporary stub implementation to satisfy gqlgen after migrating GraphQL files from reearthx/account.
// This resolver was added to avoid compile-time errors.
// Will be implemented if needed, or removed if unused after migration.
func (r *mutationResolver) VerifyUser(ctx context.Context, input gqlmodel.VerifyUserInput) (*gqlmodel.UserPayload, error) {
	return nil, nil
}

// Temporary stub implementation to satisfy gqlgen after migrating GraphQL files from reearthx/account.
// This resolver was added to avoid compile-time errors.
// Will be implemented if needed, or removed if unused after migration.
func (r *mutationResolver) FindOrCreate(ctx context.Context, input gqlmodel.FindOrCreateInput) (*gqlmodel.UserPayload, error) {
	return nil, nil
}

// Temporary stub implementation to satisfy gqlgen after migrating GraphQL files from reearthx/account.
// This resolver was added to avoid compile-time errors.
// Will be implemented if needed, or removed if unused after migration.
func (r *mutationResolver) CreateVerification(ctx context.Context, input gqlmodel.CreateVerificationInput) (*bool, error) {
	return nil, nil
}

// Temporary stub implementation to satisfy gqlgen after migrating GraphQL files from reearthx/account.
// This resolver was added to avoid compile-time errors.
// Will be implemented if needed, or removed if unused after migration.
func (r *mutationResolver) StartPasswordReset(ctx context.Context, input gqlmodel.StartPasswordResetInput) (*bool, error) {
	return nil, nil
}

// Temporary stub implementation to satisfy gqlgen after migrating GraphQL files from reearthx/account.
// This resolver was added to avoid compile-time errors.
// Will be implemented if needed, or removed if unused after migration.
func (r *mutationResolver) PasswordReset(ctx context.Context, input gqlmodel.PasswordResetInput) (*bool, error) {
	return nil, nil
}
