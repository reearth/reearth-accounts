package cerbos

import (
	"context"
	"net"
	"strings"
	"testing"

	"github.com/cerbos/cerbos-sdk-go/cerbos"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
)

// A gRPC server with no services answers Unimplemented, which is enough to
// observe the client span.
func TestNewClient_CallsProduceSpansWithoutPersonalData(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp)))

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := grpc.NewServer()
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()

	client, err := NewClient(lis.Addr().String(), false)
	require.NoError(t, err)

	const email = "span-probe@example.com"
	principal := cerbos.NewPrincipal("user-1", "reader").WithAttr("email", email)
	resource := cerbos.NewResource("workspace", "ws-1")
	_, err = client.IsAllowed(context.Background(), principal, resource, "read")
	require.Error(t, err)

	spans := exp.GetSpans()
	require.NotEmpty(t, spans)
	var found bool
	for _, s := range spans {
		if s.SpanKind == trace.SpanKindClient && strings.Contains(s.Name, "cerbos.svc.v1.CerbosService/CheckResources") {
			found = true
		}
		for _, kv := range s.Attributes {
			assert.NotContains(t, kv.Value.Emit(), email, string(kv.Key))
		}
		for _, ev := range s.Events {
			for _, kv := range ev.Attributes {
				assert.NotContains(t, kv.Value.Emit(), email, string(kv.Key))
			}
		}
	}
	assert.True(t, found, "no Cerbos client span among %v", spans)
}
