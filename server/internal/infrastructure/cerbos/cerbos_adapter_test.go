package cerbos

import (
	"context"
	"net"
	"sync"
	"testing"

	"github.com/cerbos/cerbos-sdk-go/cerbos"
	requestv1 "github.com/cerbos/cerbos/api/genpb/cerbos/request/v1"
	responsev1 "github.com/cerbos/cerbos/api/genpb/cerbos/response/v1"
	svcv1 "github.com/cerbos/cerbos/api/genpb/cerbos/svc/v1"
	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/appx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

type auxJWTRecorder struct {
	svcv1.UnimplementedCerbosServiceServer
	mu   sync.Mutex
	jwts []string
}

func (r *auxJWTRecorder) CheckResources(_ context.Context, req *requestv1.CheckResourcesRequest) (*responsev1.CheckResourcesResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jwts = append(r.jwts, req.GetAuxData().GetJwt().GetToken())
	return &responsev1.CheckResourcesResponse{RequestId: req.GetRequestId()}, nil
}

func (r *auxJWTRecorder) last() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.jwts[len(r.jwts)-1]
}

func TestCheckPermissions_ForwardsOnlyAUsersOwnTokenAsAuxJWT(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	rec := &auxJWTRecorder{}
	srv := grpc.NewServer()
	svcv1.RegisterCerbosServiceServer(srv, rec)
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()
	client, err := NewClient(lis.Addr().String(), false)
	require.NoError(t, err)
	a := NewCerbosAdapter(client)
	check := func(ctx context.Context) string {
		_, err := a.CheckPermissions(ctx, cerbos.NewPrincipal("user-1", "reader"),
			[]*cerbos.Resource{cerbos.NewResource("workspace", "ws-1")}, []string{"read"})
		require.NoError(t, err)
		return rec.last()
	}

	own := context.WithValue(context.Background(), adapter.AuthInfoKey, appx.AuthInfo{Token: "users-own-token"})
	assert.Equal(t, "users-own-token", check(own), "a user's own token")

	viaClient := (&adapter.ClientCall{
		Client:  &workspace.Client{ID: "svc", Name: "svc"},
		Subject: workspace.UserSubject(workspace.NewUserID()),
	}).Attach(context.WithValue(context.Background(), adapter.AuthInfoKey, appx.AuthInfo{Token: "client-token"}))
	assert.Empty(t, check(viaClient), "a client call")
}
