package gqlerror

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"sync/atomic"

	"github.com/reearth/reearthx/log"
)

type AccountsError error

var ErrUnauthorized AccountsError = errors.New("unauthorized")

func IsUnauthorized(err error) bool {
	return strings.Contains(err.Error(), ErrUnauthorized.Error())
}

// warnExpected is off by default so that adopting a new version of this module
// changes nothing for a service that has not asked for it. A consumer that
// wants expected failures at WARN opts in once during start up, before serving
// traffic.
var warnExpected atomic.Bool

// SetWarnExpected controls whether ReturnAccountsWarn logs at WARN. It is off
// by default, and while it is off those call sites keep logging at ERROR
// exactly as they do today.
//
// Only the severity changes: the error returned to the caller is the same
// either way, so this is safe to turn on without auditing call sites. Call it
// during start up; it is not meant to be flipped while requests are in flight.
func SetWarnExpected(v bool) { warnExpected.Store(v) }

func ReturnAccountsError(ctx context.Context, err error) AccountsError {
	_, file, line, _ := runtime.Caller(1)
	if strings.Contains(err.Error(), "401") {
		log.Warnfc(ctx, "[Warn] unauthorized at %s:%d %+v", file, line, err)
		return ErrUnauthorized
	}
	log.Errorfc(ctx, "[Error] error with caller logging at %s:%d %+v", file, line, err)
	return err
}

// ReturnAccountsWarn is for the calls whose failures are usually a rejection
// the caller caused and can correct: a workspace that does not exist, a name
// that fails validation, a member who has already joined. Those are not
// defects, and logging them at ERROR makes every consumer's alerting treat a
// user mistake as a server fault.
//
// The severity is chosen by the call site rather than by the error, so a
// genuine failure of the accounts service on one of these calls is logged at
// WARN too. That is a deliberate trade: an outage or a rejected token is
// visible at the many call sites that still use ReturnAccountsError, notably
// the user lookup on the authentication path. A call that hangs until it times
// out is the case this does not cover, so prefer ReturnAccountsError wherever a
// failure would not show up anywhere else.
func ReturnAccountsWarn(ctx context.Context, err error) AccountsError {
	_, file, line, _ := runtime.Caller(1)

	if strings.Contains(err.Error(), "401") {
		log.Warnfc(ctx, "[Warn] unauthorized at %s:%d %+v", file, line, err)
		return ErrUnauthorized
	}

	if warnExpected.Load() {
		// Same message as the ERROR path on purpose: the severity is the only
		// thing that differs, so a log query matching on the text keeps working
		// whichever way a consumer has this set.
		log.Warnfc(ctx, "[Error] error with caller logging at %s:%d %+v", file, line, err)
		return err
	}

	log.Errorfc(ctx, "[Error] error with caller logging at %s:%d %+v", file, line, err)
	return err
}
