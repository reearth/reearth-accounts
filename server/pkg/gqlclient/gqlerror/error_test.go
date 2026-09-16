package gqlerror

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/reearth/reearthx/log"
	"github.com/stretchr/testify/assert"
)

// logBuf captures what the package logs. The logger is global and reearthx
// exposes no way to read the current destination, so it is redirected once here
// rather than per call, where restoring it would mean guessing what it was.
var logBuf bytes.Buffer

func TestMain(m *testing.M) {
	log.SetOutput(&logBuf)
	os.Exit(m.Run())
}

func severityOf(t *testing.T, f func(context.Context, error) AccountsError, err error) string {
	t.Helper()

	logBuf.Reset()
	_ = f(context.Background(), err)

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

// The default has to leave a service that merely bumps this module on exactly
// the behaviour it had before, so ReturnAccountsWarn still reaches ERROR until
// the consumer opts in.
func TestWarnExpectedIsOffByDefault(t *testing.T) {
	assert.False(t, WarnExpected())

	rejection := errors.New("input: deleteWorkspace operation denied")
	assert.Equal(t, "ERROR", severityOf(t, ReturnAccountsWarn, rejection))

	SetWarnExpected(true)
	t.Cleanup(func() { SetWarnExpected(false) })

	assert.True(t, WarnExpected())
	assert.Equal(t, "WARN", severityOf(t, ReturnAccountsWarn, rejection))
}

// ReturnAccountsError is unaffected by the option: it is the choice made at
// call sites whose failures always mean the server got something wrong.
func TestReturnAccountsErrorIsAlwaysError(t *testing.T) {
	SetWarnExpected(true)
	t.Cleanup(func() { SetWarnExpected(false) })

	assert.Equal(t, "ERROR", severityOf(t, ReturnAccountsError, errors.New("input: updateProject transaction error")))
	assert.Equal(t, "ERROR", severityOf(t, ReturnAccountsError, errors.New("dial tcp: connection refused")))
}

// Severity is chosen by the call site, so a genuine failure on one of the
// opted-in calls is logged at WARN as well. This pins that known trade rather
// than leaving it to be discovered.
func TestWarnSiteAlsoDownGradesRealFailures(t *testing.T) {
	SetWarnExpected(true)
	t.Cleanup(func() { SetWarnExpected(false) })

	assert.Equal(t, "WARN", severityOf(t, ReturnAccountsWarn, errors.New("dial tcp: connection refused")))
	assert.Equal(t, "WARN", severityOf(t, ReturnAccountsWarn, errors.New("net/http: request canceled (Client.Timeout exceeded while awaiting headers)")))
}

// The value returned must not change with severity, or a consumer that switches
// on it would behave differently after this change.
func TestReturnedValueIsPreserved(t *testing.T) {
	ctx := context.Background()
	e := errors.New("input: deleteWorkspace operation denied")

	assert.Equal(t, e, error(ReturnAccountsError(ctx, e)))
	assert.Equal(t, e, error(ReturnAccountsWarn(ctx, e)))

	// Unauthorized is the one documented exception, for both.
	assert.Equal(t, ErrUnauthorized, ReturnAccountsError(ctx, errors.New("401 Unauthorized")))
	assert.Equal(t, ErrUnauthorized, ReturnAccountsWarn(ctx, errors.New("401 Unauthorized")))
}
