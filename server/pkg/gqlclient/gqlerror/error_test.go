package gqlerror

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	graphql "github.com/hasura/go-graphql-client"
	"github.com/reearth/reearthx/log"
	"github.com/stretchr/testify/assert"
)

func gqlErr(message string, ext map[string]any) graphql.Errors {
	if ext == nil {
		ext = map[string]any{}
	}
	return graphql.Errors{{Message: message, Extensions: ext}}
}

func TestIsExpected(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		// Business rejections seen in production logs, verbatim.
		{"not found", gqlErr("input: findUserByAlias not found", nil), true},
		{"invalid user name", gqlErr("input: createWorkspace invalid user name", nil), true},
		{"operation denied", gqlErr("input: deleteWorkspace operation denied", nil), true},
		{"personal workspace", gqlErr("input: deleteWorkspace personal workspace cannot be modified", nil), true},
		{"owner cannot leave", gqlErr("input: removeUserFromWorkspace owner user cannot leave from the workspace", nil), true},
		{"add users not found", gqlErr("input: addUsersToWorkspace not found", nil), true},
		{"invalid email", gqlErr("input: signup invalid email", nil), true},
		{"password policy", gqlErr("input: signup password should have numbers", nil), true},
		{"password length", gqlErr("input: signup password at least 8 characters", nil), true},
		{"already exists", gqlErr("input: signup user already exists", nil), true},
		{"invalid workspace name", gqlErr("input: createWorkspace invalid workspace name", nil), true},
		{"already joined", gqlErr("input: addUsersToWorkspace user already joined", nil), true},
		{"not a member", gqlErr("input: removeUserFromWorkspace target user does not exist in the workspace", nil), true},

		// An authorization denial is deliberately kept at ERROR so that a burst
		// of them stays visible.
		{"permission denied", gqlErr("input: createWorkspace permission denied", nil), false},

		// A malformed query never reaches a resolver. This is the shape the
		// signup document bug took, and it has to stay visible.
		{"request error", gqlErr(`Variable "$id" is not defined.`, map[string]any{"code": graphql.ErrRequestError}), false},
		{"internal extension", gqlErr("not found", map[string]any{"internal": "boom"}), false},
		{"internal in message", gqlErr("internal error: user not found", nil), false},

		// Genuine defects.
		{"transaction", gqlErr("input: updateProject transaction error", nil), false},
		{"plain error", errors.New("dial tcp: connection refused"), false},
		{"empty list", graphql.Errors{}, false},

		// A list is only expected when every entry is.
		{"mixed list", graphql.Errors{
			{Message: "input: signup invalid email", Extensions: map[string]any{}},
			{Message: "input: signup transaction error", Extensions: map[string]any{}},
		}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isExpected(tt.err))
		})
	}
}

// The returned value must not change with severity, or consumers that switch on
// it would behave differently after this change.
func TestReturnAccountsErrorPreservesValue(t *testing.T) {
	ctx := context.Background()

	expected := gqlErr("input: signup invalid email", nil)
	assert.Equal(t, error(expected), error(ReturnAccountsError(ctx, expected)))

	defect := gqlErr("input: updateProject transaction error", nil)
	assert.Equal(t, error(defect), error(ReturnAccountsError(ctx, defect)))

	// Unauthorized is the one documented exception.
	assert.Equal(t, ErrUnauthorized, ReturnAccountsError(ctx, errors.New("401 Unauthorized")))
}

// The default has to leave a service that merely bumps this module on exactly
// the behaviour it had before, so an expected failure still reaches ERROR until
// the consumer opts in.
func TestClassifyExpectedIsOffByDefault(t *testing.T) {
	assert.False(t, ClassifyExpected())

	expected := gqlErr("input: signup invalid email", nil)

	assert.Equal(t, "ERROR", severityOf(t, expected))

	SetClassifyExpected(true)
	t.Cleanup(func() { SetClassifyExpected(false) })

	assert.True(t, ClassifyExpected())
	assert.Equal(t, "WARN", severityOf(t, expected))

	// A defect stays at ERROR either way.
	assert.Equal(t, "ERROR", severityOf(t, gqlErr("input: updateProject transaction error", nil)))
}

// logBuf captures what the package logs. The logger is global and reearthx
// exposes no way to read the current destination, so it is redirected once here
// rather than per call, where restoring it would mean guessing what it was.
var logBuf bytes.Buffer

func TestMain(m *testing.M) {
	log.SetOutput(&logBuf)
	os.Exit(m.Run())
}

// severityOf captures the level ReturnAccountsError logs err at.
func severityOf(t *testing.T, err error) string {
	t.Helper()

	logBuf.Reset()

	_ = ReturnAccountsError(context.Background(), err)

	out := logBuf.String()
	switch {
	case strings.Contains(out, "ERROR"):
		return "ERROR"
	case strings.Contains(out, "WARN"):
		return "WARN"
	default:
		return "NONE: " + out
	}
}
