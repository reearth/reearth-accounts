package auth0

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestAuth0_CallsProduceSpansWithoutPersonalData(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp)))

	const sub = "auth0|span-probe-subject"
	const email = "span-probe@example.com"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"email":"` + email + `"}`))
	}))
	defer srv.Close()
	a := &Auth0{domain: srv.URL, client: srv.Client(), disableLogging: true}

	_, err := a.exec(context.Background(), http.MethodPatch, "api/v2/users/"+sub, "", map[string]string{"email": email})
	require.NoError(t, err)
	require.NoError(t, a.execInto(context.Background(), http.MethodGet, "api/v2/users/"+sub, "", nil, &map[string]any{}))

	spans := exp.GetSpans()
	require.Len(t, spans, 2)
	assert.Equal(t, "auth0 PATCH api/v2/users/{id}", spans[0].Name)
	assert.Equal(t, "auth0 GET api/v2/users/{id}", spans[1].Name)
	for _, s := range spans {
		assert.Equal(t, trace.SpanKindClient, s.SpanKind)
		for _, kv := range s.Attributes {
			v := kv.Value.Emit()
			assert.False(t, strings.Contains(v, sub) || strings.Contains(v, email), "%s=%s", kv.Key, v)
		}
		assert.False(t, strings.Contains(s.Name, sub))
	}
}
