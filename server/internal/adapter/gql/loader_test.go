package gql

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/adapter/gql/gqlmodel"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type alignedUserUsecase struct {
	interfaces.User
}

func (alignedUserUsecase) FetchByID(_ context.Context, ids user.IDList) (user.List, error) {
	return make(user.List, len(ids)), nil
}

func TestUserLoader_FetchByID_MissingUser(t *testing.T) {
	got, err := NewUserLoader(alignedUserUsecase{}).FetchByID(context.Background(), gqlmodel.IDFrom(user.NewID()))
	assert.NoError(t, err)
	assert.Nil(t, got)
}

type reorderingWorkspaceUsecase struct {
	interfaces.Workspace
	res workspace.List
}

func (u reorderingWorkspaceUsecase) Fetch(context.Context, workspace.IDList, *workspace.Operator) (workspace.List, error) {
	return u.res, nil
}

func TestWorkspaceLoader_Fetch_AlignsWithKeys(t *testing.T) {
	a := workspace.New().NewID().Name("a").MustBuild()
	b := workspace.New().NewID().Name("b").MustBuild()
	keys := []gqlmodel.ID{gqlmodel.IDFrom(b.ID()), gqlmodel.IDFrom(workspace.NewID()), gqlmodel.IDFrom(a.ID())}

	got, errs := NewWorkspaceLoader(reorderingWorkspaceUsecase{res: workspace.List{a, nil, b}}, nil).Fetch(context.Background(), keys)
	require.Empty(t, errs)
	require.Len(t, got, 3)
	assert.Equal(t, "b", got[0].Name)
	assert.Nil(t, got[1])
	assert.Equal(t, "a", got[2].Name)
}
