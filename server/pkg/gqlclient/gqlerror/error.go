package gqlerror

import (
	"context"
	"errors"
	"strings"

	"github.com/reearth/reearth-accounts/server/pkg/applog"
)

type AccountsError error

var ErrUnauthorized AccountsError = errors.New("unauthorized")

func IsUnauthorized(err error) bool {
	return strings.Contains(err.Error(), ErrUnauthorized.Error())
}

func ReturnAccountsError(ctx context.Context, err error) AccountsError {
	if strings.Contains(err.Error(), "401") {
		applog.WarnDepth(ctx, 1, "accounts: unauthorized", applog.Err(err))
		return ErrUnauthorized
	}
	applog.ErrorDepth(ctx, 1, "accounts: request failed", applog.Err(err))
	return err
}

func ReturnAccountsWarn(ctx context.Context, err error) AccountsError {
	if strings.Contains(err.Error(), "401") {
		applog.WarnDepth(ctx, 1, "accounts: unauthorized", applog.Err(err))
		return ErrUnauthorized
	}
	applog.WarnDepth(ctx, 1, "accounts: transport failure", applog.Err(err))
	return err
}
