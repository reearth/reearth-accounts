package gql

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/internal/adapter/gql/gqlmodel"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type allowAllCerbos struct{}

func (allowAllCerbos) CheckPermission(context.Context, user.ID, interfaces.CheckPermissionParam) (*interfaces.CheckPermissionResult, error) {
	return &interfaces.CheckPermissionResult{Allowed: true}, nil
}

func TestCheckPermission_RefusesClientCalls(t *testing.T) {
	u := user.New().NewID().Name("u").Email("u@example.com").MustBuild()
	uid := u.ID()
	iid := id.NewIntegrationID()
	svc := &workspace.Client{ID: "svc", Name: "svc"}
	bg := AttachUsecases(context.Background(), &interfaces.Container{Cerbos: allowAllCerbos{}}, adapter.PhotoSigner{}, false)
	r := &queryResolver{&Resolver{}}
	call := func(ctx context.Context) (*gqlmodel.CheckPermissionPayload, error) {
		return r.CheckPermission(ctx, gqlmodel.CheckPermissionInput{Service: "s", Resource: "r", Action: "read"})
	}
	actingForUser := (&adapter.ClientCall{
		Client: svc, Subject: workspace.UserSubject(uid), User: u,
		Operator: &workspace.Operator{User: &uid, Client: svc},
	}).Attach(bg)
	actingForIntegration := (&adapter.ClientCall{
		Client: svc, Subject: workspace.IntegrationSubject(iid),
		Operator: &workspace.Operator{Integration: &iid, Client: svc},
	}).Attach(bg)
	plainUser := adapter.AttachOperator(adapter.AttachUser(bg, u), &workspace.Operator{User: &uid})

	_, err := call(actingForUser)
	assert.ErrorIs(t, err, interfaces.ErrInvalidOperator, "a client acting for a user")
	_, err = call(actingForIntegration)
	assert.ErrorIs(t, err, interfaces.ErrInvalidOperator, "a client acting for an integration")
	res, err := call(plainUser)
	require.NoError(t, err, "a user's own token")
	assert.True(t, res.Allowed)
}
