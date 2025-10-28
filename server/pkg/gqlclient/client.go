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

func NewClient(host string, timeout int, transport http.RoundTripper) *Client {
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
