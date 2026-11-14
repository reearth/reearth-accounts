package applog

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/reearth/reearthx/log"
	"github.com/stretchr/testify/assert"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stdout) })
	return &buf
}

func TestEntriesCarryTheCurrentSpan(t *testing.T) {
	ctx, span := sdktrace.NewTracerProvider().Tracer("t").Start(context.Background(), "op")
	defer span.End()
	sc := span.SpanContext()

	t.Run("with a GCP project, Cloud Logging keys", func(t *testing.T) {
		prev := log.GCPProject
		log.GCPProject = "probe-project"
		t.Cleanup(func() { log.GCPProject = prev })
		buf := captureLog(t)

		Info(ctx, "probe")

		assert.Contains(t, buf.String(), `"logging.googleapis.com/trace": "projects/probe-project/traces/`+sc.TraceID().String()+`"`)
		assert.Contains(t, buf.String(), `"logging.googleapis.com/spanId": "`+sc.SpanID().String()+`"`)
	})

	t.Run("without a project, plain IDs", func(t *testing.T) {
		t.Setenv("GOOGLE_CLOUD_PROJECT", "")
		buf := captureLog(t)

		Info(ctx, "probe")

		assert.Contains(t, buf.String(), `"trace_id": "`+sc.TraceID().String()+`"`)
		assert.Contains(t, buf.String(), `"span_id": "`+sc.SpanID().String()+`"`)
	})

	t.Run("no span, no trace keys", func(t *testing.T) {
		buf := captureLog(t)
		Info(context.Background(), "probe")
		assert.NotContains(t, buf.String(), "trace")
	})
}
