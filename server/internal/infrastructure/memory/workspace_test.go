package memory

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/pkg/id"
	"github.com/reearth/reearth-accounts/pkg/workspace"
	"github.com/reearth/reearthx/rerror"
	"github.com/stretchr/testify/assert"
)

func TestNewWorkspace(t *testing.T) {
	ws := NewWorkspace()
	assert.NotNil(t, ws)
}

func TestWorkspace_Find(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		wid := id.NewWorkspaceID()
		ws, err := workspace.New().
			ID(wid).
			Name("Test Workspace").
			Build()
		assert.NoError(t, err)

		repo := NewWorkspaceWith(ws)
		got, err := repo.FindByID(ctx, wid)
		assert.NoError(t, err)
		assert.Equal(t, ws, got)
	})

	t.Run("not found", func(t *testing.T) {
		repo := NewWorkspace()
		_, err := repo.FindByID(ctx, id.NewWorkspaceID())
		assert.Error(t, err)
		assert.EqualError(t, rerror.ErrNotFound, err.Error())
	})
}
