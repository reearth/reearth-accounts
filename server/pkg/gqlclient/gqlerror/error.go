package gqlerror

import (
	"context"
	"errors"
	"runtime"
	"strings"

	"github.com/reearth/reearthx/log"
	"github.com/reearth/reearthx/rerror"
)

type AccountsError error

var ErrUnauthorized AccountsError = errors.New("unauthorized")

var ErrNotFound AccountsError = rerror.ErrNotFound

func IsUnauthorized(err error) bool {
	return strings.Contains(err.Error(), ErrUnauthorized.Error())
}

func IsNotFound(err error) bool {
	return strings.Contains(err.Error(), "not found")
}

func ReturnAccountsError(ctx context.Context, err error) AccountsError {
	_, file, line, _ := runtime.Caller(1)
	if strings.Contains(err.Error(), "401") {
		log.Warnfc(ctx, "[Warn] unauthorized at %s:%d %+v", file, line, err)
		return ErrUnauthorized
	}
	if IsNotFound(err) {
		log.Debugfc(ctx, "[Debug] not found at %s:%d %+v", file, line, err)
		return ErrNotFound
	}
	log.Errorfc(ctx, "[Error] error with caller logging at %s:%d %+v", file, line, err)
	return err
}
