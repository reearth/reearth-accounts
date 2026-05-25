package gqlerror

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"

	"github.com/hasura/go-graphql-client"
	"github.com/reearth/reearthx/log"
	"github.com/reearth/reearthx/rerror"
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
	if isNotFoundError(err) {
		log.Warnfc(ctx, "[Warn] not found at %s:%d %+v", file, line, err)
		return fmt.Errorf("%w: %v", rerror.ErrNotFound, err)
	}
	return err
}

func isNotFoundError(err error) bool {
	var gqlErrs graphql.Errors
	if errors.As(err, &gqlErrs) {
		for _, gqlErr := range gqlErrs {
			if strings.Contains(gqlErr.Message, "not found") {
				return true
			}
		}
	}
	return false
}
