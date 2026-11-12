package e2e

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestTracing_SpansCarryOperationButNoPersonalData(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp)))
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	e, _ := StartServer(t, &app.Config{Mock_Auth: true, OtelEnabled: true}, false, seedLogProbe)

	b, err := json.Marshal(GraphQLRequest{
		OperationName: "Probe",
		Query: `query Probe($q: String!) {
			me { id email metadata { description website } }
			byEmail: userByNameOrEmail(nameOrEmail: $q) { id }
		}`,
		Variables: map[string]any{"q": logProbeOtherEmail},
	})
	require.NoError(t, err)
	e.POST("/api/graphql").WithHeader("Content-Type", "application/json").
		WithHeader("X-Internal-Service", "visualizer").WithBytes(b).
		Expect().Status(http.StatusOK).Body().Contains(logProbeSelfValues[0])
	e.GET("/api/users/by-name-or-email").WithQuery("q", logProbeOtherEmail).Expect().Status(http.StatusOK)

	spans := exp.GetSpans()
	require.NotEmpty(t, spans)
	forbidden := append([]string{logProbeOtherEmail, url.QueryEscape(logProbeOtherEmail)}, logProbeSelfValues...)

	var op *tracetest.SpanStub
	for i, s := range spans {
		if s.Name == "GraphQL query Probe" {
			op = &spans[i]
		}
		for _, kv := range s.Attributes {
			for _, v := range forbidden {
				assert.NotContains(t, kv.Value.Emit(), v, "span %q attribute %s", s.Name, kv.Key)
			}
		}
	}
	require.NotNil(t, op, "no GraphQL operation span")
	attrs := map[attribute.Key]attribute.Value{}
	for _, kv := range op.Attributes {
		attrs[kv.Key] = kv.Value
	}
	assert.Equal(t, []string{"me", "userByNameOrEmail"}, attrs["graphql.operation.fields"].AsStringSlice())
	assert.Equal(t, restDemoUID.String(), attrs["enduser.id"].AsString())
	assert.Equal(t, "visualizer", attrs["accounts.caller"].AsString())
}
