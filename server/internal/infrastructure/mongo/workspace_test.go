package mongo

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/pkg/id"
	"github.com/reearth/reearth-accounts/pkg/workspace"
	"github.com/reearth/reearthx/mongox"
	"github.com/stretchr/testify/assert"
)

func TestWorkspace_FindByID(t *testing.T) {
	c := Connect(t)(t)
	ctx := context.Background()
	client := mongox.NewClientWithDatabase(c)
	assert.NotNil(t, client)

	t.Run("success", func(t *testing.T) {
		rWorkspace := NewWorkspace(client)
		metadata := workspace.NewMetadata()
		metadata.SetDescription("Test description")
		metadata.SetWebsite("https://example.com")

		ws, err := workspace.New().
			ID(id.NewWorkspaceID()).
			Name("Test Workspace").
			Alias("test-alias").
			Metadata(metadata).
			Build()
		assert.NoError(t, err)
		err = rWorkspace.Save(ctx, ws)
		assert.NoError(t, err)

		got, err := rWorkspace.FindByID(ctx, ws.ID())
		assert.NoError(t, err)
		assert.NotNil(t, got)
		assert.Equal(t, ws.ID(), got.ID())
		assert.Equal(t, ws.Name(), got.Name())
		assert.Equal(t, ws.Alias(), got.Alias())
		assert.NotNil(t, got.Metadata())
		assert.Equal(t, ws.Metadata().Description(), got.Metadata().Description())
		assert.Equal(t, ws.Metadata().Website(), got.Metadata().Website())
	})
}
