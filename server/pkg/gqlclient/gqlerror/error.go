package gqlerror

import (
	"context"
	"errors"
	"runtime"
	"strings"

	"github.com/reearth/reearthx/log"
)

type AccountsError error

var ErrUnauthorized AccountsError = errors.New("unauthorized")

func IsUnauthorized(err error) bool {
	return strings.Contains(err.Error(), ErrUnauthorized.Error())
}

func ReturnAccountsError(ctx context.Context, err error) AccountsError {
	_, file, line, _ := runtime.Caller(1)
	log.Errorfc(ctx, "[Error] error with caller logging at %s:%d %+v", file, line, err)
	if strings.Contains(err.Error(), "401") {
		return ErrUnauthorized
	}
	return err
}
