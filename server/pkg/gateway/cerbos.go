package gateway

import (
	"context"

	"github.com/cerbos/cerbos-sdk-go/cerbos"
)

//go:generate mockgen -source=./cerbos.go -destination=./mock_gateway/mock_cerbos.go -package mock_gateway
type CerbosGateway interface {
	CheckPermissions(ctx context.Context, principal *cerbos.Principal, resources []*cerbos.Resource, actions []string) (*cerbos.CheckResourcesResponse, error)
}
