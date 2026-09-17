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

// ReturnAccountsWarn is ReturnAccountsError at WARN, for the calls whose
// failures are usually a rejection the caller caused and can correct: a
// workspace that does not exist, a name that fails validation, a member who has
// already joined. Those are not defects, and logging them at ERROR makes a
// consumer's alerting treat a user mistake as a server fault.
//
// The severity belongs to the call site rather than the error, so a genuine
// failure of one of these calls is logged at WARN too. An outage or a rejected
// token still reaches ERROR through the calls that use ReturnAccountsError,
// notably the user lookup on the authentication path. A call that hangs until
// it times out is the case this does not cover, so prefer ReturnAccountsError
// wherever a failure would not show up anywhere else.
//
// The body is a copy rather than a call into a shared helper because
// runtime.Caller(1) has to see the real call site.
func ReturnAccountsWarn(ctx context.Context, err error) AccountsError {
	_, file, line, _ := runtime.Caller(1)
	if strings.Contains(err.Error(), "401") {
		log.Warnfc(ctx, "[Warn] unauthorized at %s:%d %+v", file, line, err)
		return ErrUnauthorized
	}
	// Same message as ReturnAccountsError on purpose: the severity is the only
	// difference, so a log query matching on the text keeps working.
	log.Warnfc(ctx, "[Error] error with caller logging at %s:%d %+v", file, line, err)
	return err
}
