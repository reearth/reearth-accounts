package gqlclient

import (
	"net/http"
	"strings"
	"time"

	"github.com/hasura/go-graphql-client"
	"github.com/reearth/reearth-accounts/server/pkg/gqlclient/cerbos"
	"github.com/reearth/reearth-accounts/server/pkg/gqlclient/user"
	"github.com/reearth/reearth-accounts/server/pkg/gqlclient/workspace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type Client struct {
	CerbosRepo    cerbos.Repo
	UserRepo      user.Repo
	WorkspaceRepo workspace.WorkspaceRepo
}

type InternalService string

const (
	InternalServiceCMSAPI        InternalService = "cms-api"
	InternalServiceFlowAPI       InternalService = "flow-api"
	InternalServiceDashboardAPI  InternalService = "dashboard-api"
	InternalServiceVisualizerAPI InternalService = "visualizer-api"
	InternalServiceUnknown       InternalService = "unknown"
)

type AccountsTransport struct {
	serviceRoundTripper http.RoundTripper
	internalService     InternalService
}

func NewAccountsTransport(serviceRoundTripper http.RoundTripper, internalService InternalService) *AccountsTransport {
	return &AccountsTransport{serviceRoundTripper: serviceRoundTripper, internalService: internalService}
}

func (t *AccountsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.Header.Set("X-Internal-Service", string(t.internalService))
	return t.serviceRoundTripper.RoundTrip(req2)
}

func NewClient(
	host string,
	timeout int,
	transport http.RoundTripper,
) *Client {
	httpClient := &http.Client{
		Transport: traceContextTransport{next: transport},
		Timeout:   time.Duration(timeout) * time.Second,
	}

	normalizedHost := strings.TrimRight(host, "/")
	fullEndpoint := normalizedHost + "/api/graphql"
	gqlClient := graphql.NewClient(fullEndpoint, httpClient)

	return &Client{
		CerbosRepo:    cerbos.NewRepo(gqlClient),
		UserRepo:      user.NewRepo(gqlClient),
		WorkspaceRepo: workspace.NewRepo(gqlClient),
	}
}

// traceContextTransport sends the caller's span context as a W3C traceparent
// header, so the accounts trace continues the caller's (ADR-0003 decision 10).
// It uses the W3C propagator directly because callers may not set a global one.
type traceContextTransport struct {
	next http.RoundTripper
}

func (t traceContextTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	next := t.next
	if next == nil {
		next = http.DefaultTransport
	}
	if !trace.SpanContextFromContext(req.Context()).IsValid() {
		return next.RoundTrip(req)
	}
	req = req.Clone(req.Context())
	propagation.TraceContext{}.Inject(req.Context(), propagation.HeaderCarrier(req.Header))
	return next.RoundTrip(req)
}
