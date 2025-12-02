package cerbos

import (
	"context"

	"github.com/cerbos/cerbos-sdk-go/cerbos"
	"github.com/reearth/reearth-accounts/server/internal/adapter"
)

type CerbosAdapter struct {
	client *cerbos.GRPCClient
}

func NewCerbosAdapter(client *cerbos.GRPCClient) *CerbosAdapter {
	return &CerbosAdapter{client: client}
}

func (c *CerbosAdapter) CheckPermissions(ctx context.Context, principal *cerbos.Principal, resources []*cerbos.Resource, actions []string) (*cerbos.CheckResourcesResponse, error) {
	batch := cerbos.NewResourceBatch()
	for _, resource := range resources {
		batch.Add(resource, actions...)
	}

	authInfo := adapter.GetAuthInfo(ctx)
	var token string
	if authInfo != nil {
		token = authInfo.Token
	}

	return c.client.With(cerbos.AuxDataJWT(token, "test")).CheckResources(ctx, principal, batch)
}
