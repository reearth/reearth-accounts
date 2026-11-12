package gql

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/adapter/gql/gqlmodel"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/stretchr/testify/assert"
)

func TestCheckPermission_NoUser(t *testing.T) {
	r := &queryResolver{&Resolver{}}
	res, err := r.CheckPermission(context.Background(), gqlmodel.CheckPermissionInput{Service: "s", Resource: "r", Action: "read"})
	assert.Nil(t, res)
	assert.ErrorIs(t, err, interfaces.ErrInvalidOperator)
}

func TestCreateWorkspace_NoUser(t *testing.T) {
	r := &mutationResolver{&Resolver{}}
	res, err := r.CreateWorkspace(context.Background(), gqlmodel.CreateWorkspaceInput{Alias: "a", Name: "n"})
	assert.Nil(t, res)
	assert.ErrorIs(t, err, interfaces.ErrInvalidOperator)
}
