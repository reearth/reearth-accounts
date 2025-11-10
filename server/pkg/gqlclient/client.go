package gqlclient

import (
	"net/http"
	"strings"
	"time"

	"github.com/hasura/go-graphql-client"
	"github.com/reearth/reearth-accounts/server/pkg/gqlclient/user"
	"github.com/reearth/reearth-accounts/server/pkg/gqlclient/workspace"
)

type Client struct {
	UserRepo      user.UserRepo
	WorkspaceRepo workspace.WorkspaceRepo
}

type InternalService string

const (
	InternalServiceCMSAPI        InternalService = "cms-api"
	InternalServiceFlowAPI       InternalService = "flow-api"
	InternalServiceDashboardAPI  InternalService = "dashboard-api"
	InternalServiceVisualizerAPI InternalService = "visualizer-api"
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
		Transport: transport,
		Timeout:   time.Duration(timeout) * time.Second,
	}

	normalizedHost := strings.TrimRight(host, "/")
	fullEndpoint := normalizedHost + "/api/graphql"
	gqlClient := graphql.NewClient(fullEndpoint, httpClient)

	return &Client{
		UserRepo:      user.NewRepo(gqlClient),
		WorkspaceRepo: workspace.NewRepo(gqlClient),
	}
}
