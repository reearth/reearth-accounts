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
	if strings.Contains(err.Error(), "401") {
		log.Warnfc(ctx, "[Warn] unauthorized at %s:%d %+v", file, line, err)
		return ErrUnauthorized
	}
	log.Errorfc(ctx, "[Error] error with caller logging at %s:%d %+v", file, line, err)
	return err
}
