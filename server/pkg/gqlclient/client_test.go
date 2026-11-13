package gqlclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestClient_SendsW3CTraceContext(t *testing.T) {
	var mu sync.Mutex
	var traceparent []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		traceparent = append(traceparent, r.Header.Get("traceparent"))
		mu.Unlock()
		_, _ = w.Write([]byte(`{"data":{"me":null}}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, 5, NewAccountsTransport(http.DefaultTransport, InternalServiceCMSAPI))

	// The caller never sets a global propagator; the client still sends W3C.
	ctx, span := sdktrace.NewTracerProvider().Tracer("caller").Start(context.Background(), "caller")
	_, _ = c.UserRepo.FindMe(ctx)
	span.End()
	_, _ = c.UserRepo.FindMe(context.Background())

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, traceparent, 2)
	sc := span.SpanContext()
	assert.Equal(t, "00-"+sc.TraceID().String()+"-"+sc.SpanID().String()+"-01", traceparent[0])
	assert.Empty(t, traceparent[1], "no span in the context, no header")
}
