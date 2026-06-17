package gqlerror

import (
	"context"
	"errors"
	"runtime"
	"strings"

	"github.com/hasura/go-graphql-client"
	"github.com/reearth/reearthx/log"
)

type AccountsError error

var ErrUnauthorized AccountsError = errors.New("unauthorized")

var ErrNotFound AccountsError = errors.New("not found")

func IsUnauthorized(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), ErrUnauthorized.Error())
}

func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrNotFound) {
		return true
	}
	var gqlErrs graphql.Errors
	if errors.As(err, &gqlErrs) {
		for _, gqlErr := range gqlErrs {
			if strings.Contains(strings.ToLower(gqlErr.Message), "not found") {
				return true
			}
		}
	}
	return false
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
