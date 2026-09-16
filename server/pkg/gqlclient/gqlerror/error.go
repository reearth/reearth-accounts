package gqlerror

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"sync/atomic"

	graphql "github.com/hasura/go-graphql-client"
	"github.com/reearth/reearthx/log"
)

type AccountsError error

var ErrUnauthorized AccountsError = errors.New("unauthorized")

func IsUnauthorized(err error) bool {
	return strings.Contains(err.Error(), ErrUnauthorized.Error())
}

// expectedMessages are the business rejections a caller causes and can correct:
// the request reached a resolver and was refused on its merits. They are logged
// at WARN so that a caller mistake does not read as a server defect.
//
// The client only ever sees a flattened message, because the schema attaches no
// error code to these, so the match is on text. Each entry is the message of an
// error value this module exports, listed beside it:
//
//	"not found"                                  rerror.ErrNotFound and the resolvers that wrap it
//	"already exists"                             interfaces.ErrUserAlreadyExists
//	"invalid email"                              user.ErrInvalidEmail, interfaces.ErrInvalidUserEmail
//	"invalid user name"                          user.ErrInvalidName
//	"invalid password"                           user.ErrInvalidPassword
//	"invalid secret"                             interfaces.ErrSignupInvalidSecret
//	"invalid params"                             rerror.ErrInvalidParams
//	"password at least 8 characters"             user.ErrPasswordLength
//	"password should have ..."                   user.ErrPasswordUpper/Lower/Number
//	"operation denied"                           the authorization layer
//	"personal workspace cannot be modified"      the workspace resolvers
//	"owner user cannot leave from the workspace" the workspace resolvers
var expectedMessages = []string{
	"not found",
	"already exists",
	"invalid email",
	"invalid user name",
	"invalid password",
	"invalid secret",
	"invalid params",
	"password at least 8 characters",
	"password should have",
	"operation denied",
	"personal workspace cannot be modified",
	"owner user cannot leave from the workspace",
}

// isExpected reports whether err is a business rejection rather than a defect.
//
// A transport or encoding failure is never expected, even when its message
// happens to contain one of the phrases above: those signal that the request
// never reached a resolver, which is the shape a malformed query takes, and
// keeping them at ERROR is what makes a broken client visible.
func isExpected(err error) bool {
	var (
		list graphql.Errors
		one  graphql.Error
	)

	switch {
	case errors.As(err, &list):
		if len(list) == 0 {
			return false
		}
		for _, e := range list {
			if !isExpectedOne(e) {
				return false
			}
		}
		return true
	case errors.As(err, &one):
		return isExpectedOne(one)
	}

	return false
}

func isExpectedOne(e graphql.Error) bool {
	// Transport-level failures carry a code; server detail arrives in
	// "internal". Either means this is not a caller mistake.
	if code, ok := e.Extensions["code"].(string); ok && code == graphql.ErrRequestError {
		return false
	}
	if _, ok := e.Extensions["internal"]; ok {
		return false
	}

	message := strings.ToLower(e.Message)
	// A message match cannot tell "not found" from "internal error: ... not
	// found", so anything announcing itself as internal is excluded first.
	if strings.Contains(message, "internal") {
		return false
	}

	for _, m := range expectedMessages {
		if strings.Contains(message, m) {
			return true
		}
	}

	return false
}

// classifyExpected is off by default so that adopting a new version of this
// module changes nothing for a service that has not asked for it. A consumer
// that wants expected failures at WARN opts in once during start up, before
// serving traffic.
var classifyExpected atomic.Bool

// SetClassifyExpected controls whether a business rejection is logged at WARN
// instead of ERROR. It is off unless a consumer turns it on.
//
// Only the severity changes: the error returned to the caller is the same
// either way, so this is safe to turn on without auditing call sites. Call it
// during start up; it is not meant to be flipped while requests are in flight.
func SetClassifyExpected(v bool) { classifyExpected.Store(v) }

// ClassifyExpected reports whether expected failures are logged at WARN.
func ClassifyExpected() bool { return classifyExpected.Load() }

// ReturnAccountsError logs err and returns it for the caller to handle. Only the
// log severity depends on the kind of error: the value returned is unchanged
// apart from an unauthorized response, so a consumer's behaviour does not
// change with it.
func ReturnAccountsError(ctx context.Context, err error) AccountsError {
	_, file, line, _ := runtime.Caller(1)

	if strings.Contains(err.Error(), "401") {
		log.Warnfc(ctx, "[Warn] unauthorized at %s:%d %+v", file, line, err)
		return ErrUnauthorized
	}

	if classifyExpected.Load() && isExpected(err) {
		log.Warnfc(ctx, "[Warn] expected failure at %s:%d %+v", file, line, err)
		return err
	}

	log.Errorfc(ctx, "[Error] error with caller logging at %s:%d %+v", file, line, err)
	return err
}
