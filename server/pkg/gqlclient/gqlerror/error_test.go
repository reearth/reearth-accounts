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

func TestSeverityFollowsTheFunction(t *testing.T) {
	rejection := errors.New("input: deleteWorkspace operation denied")

	assert.Equal(t, "ERROR", severityOf(t, ReturnAccountsError, rejection))
	assert.Equal(t, "WARN", severityOf(t, ReturnAccountsWarn, rejection))
}

// Severity is chosen by the call site, so a genuine failure on one of the calls
// that warns is logged at WARN as well. This pins that known trade rather than
// leaving it to be discovered.
func TestWarnAlsoDownGradesRealFailures(t *testing.T) {
	assert.Equal(t, "WARN", severityOf(t, ReturnAccountsWarn, errors.New("dial tcp: connection refused")))
	assert.Equal(t, "WARN", severityOf(t, ReturnAccountsWarn, errors.New("net/http: request canceled (Client.Timeout exceeded while awaiting headers)")))
}

// The message must not change with the severity, or a log query matching on the
// text would silently stop finding these.
func TestMessageIsTheSameAtBothSeverities(t *testing.T) {
	ctx := context.Background()
	e := errors.New("input: deleteWorkspace operation denied")

	logBuf.Reset()
	_ = ReturnAccountsError(ctx, e)
	atError := logBuf.String()

	logBuf.Reset()
	_ = ReturnAccountsWarn(ctx, e)
	atWarn := logBuf.String()

	assert.Contains(t, atError, "error with caller logging")
	assert.Contains(t, atWarn, "error with caller logging")
	assert.Contains(t, atError, "ERROR")
	assert.Contains(t, atWarn, "WARN")
}

// The value returned must not change with severity, or a consumer that switches
// on it would behave differently depending on which function a call site uses.
func TestReturnedValueIsPreserved(t *testing.T) {
	ctx := context.Background()
	e := errors.New("input: deleteWorkspace operation denied")

	assert.Equal(t, e, error(ReturnAccountsError(ctx, e)))
	assert.Equal(t, e, error(ReturnAccountsWarn(ctx, e)))

	// Unauthorized is the one documented exception, for both.
	assert.Equal(t, ErrUnauthorized, ReturnAccountsError(ctx, errors.New("401 Unauthorized")))
	assert.Equal(t, ErrUnauthorized, ReturnAccountsWarn(ctx, errors.New("401 Unauthorized")))
}
