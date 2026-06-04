package cerbos

import (
	"context"

	"github.com/cerbos/cerbos-sdk-go/cerbos"
	sharedauth "github.com/reearth/reearth-accounts/server/internal/shared/auth"
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

	authInfo := sharedauth.GetAuthInfo(ctx)
	if authInfo != nil {
		return c.client.With(cerbos.AuxDataJWT(authInfo.Token, "jwt")).CheckResources(ctx, principal, batch)
	}

	return c.client.CheckResources(ctx, principal, batch)
}
