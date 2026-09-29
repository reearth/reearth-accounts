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

func TestWarnAlsoDownGradesRealFailures(t *testing.T) {
	assert.Equal(t, "WARN", severityOf(t, ReturnAccountsWarn, errors.New("dial tcp: connection refused")))
	assert.Equal(t, "WARN", severityOf(t, ReturnAccountsWarn, errors.New("net/http: request canceled (Client.Timeout exceeded while awaiting headers)")))
}

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

func TestReturnedValueIsPreserved(t *testing.T) {
	ctx := context.Background()
	e := errors.New("input: deleteWorkspace operation denied")

	assert.Equal(t, e, error(ReturnAccountsError(ctx, e)))
	assert.Equal(t, e, error(ReturnAccountsWarn(ctx, e)))

	assert.Equal(t, ErrUnauthorized, ReturnAccountsError(ctx, errors.New("401 Unauthorized")))
	assert.Equal(t, ErrUnauthorized, ReturnAccountsWarn(ctx, errors.New("401 Unauthorized")))
}
