package cerbos

import (
	"github.com/cerbos/cerbos-sdk-go/cerbos"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
)

// NewClient connects to Cerbos with a gRPC client span per call
// (ADR-0003 decision 10). Spans carry the RPC method, not the request body.
func NewClient(host string, useSSL bool) (*cerbos.GRPCClient, error) {
	opts := []cerbos.Opt{cerbos.WithStatsHandler(otelgrpc.NewClientHandler())}
	if !useSSL {
		opts = append(opts, cerbos.WithPlaintext(), cerbos.WithTLSInsecure())
	}
	return cerbos.New(host, opts...)
}
