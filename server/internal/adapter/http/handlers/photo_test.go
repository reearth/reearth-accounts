package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/internal/adapter/http/httpmodel"
	gatewaymock "github.com/reearth/reearth-accounts/server/internal/usecase/gateway/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestSignWorkspacePhotos(t *testing.T) {
	storage := gatewaymock.NewMockStorage(gomock.NewController(t))
	storage.EXPECT().GetSignedURL(gomock.Any(), "workspaces/a.png").Return("https://signed/a", nil)
	storage.EXPECT().GetSignedURL(gomock.Any(), "workspaces/b.png").Return("workspaces/b.png", errors.New("gcs down"))
	storage.EXPECT().GetSignedURL(gomock.Any(), "workspaces/c.png").Return("https://signed/c", nil)
	photos, err := adapter.NewPhotoSigner(storage)
	require.NoError(t, err)

	withPhoto := func(p string) *httpmodel.WorkspaceResponse {
		return &httpmodel.WorkspaceResponse{Metadata: &httpmodel.WorkspaceMetadataResponse{PhotoURL: p}}
	}
	res := []*httpmodel.WorkspaceResponse{
		withPhoto("workspaces/a.png"),
		withPhoto("workspaces/b.png"),
		withPhoto(""),
		nil,
		{},
		withPhoto("workspaces/c.png"),
	}

	signWorkspacePhotos(context.Background(), photos, res)

	assert.Equal(t, "https://signed/a", res[0].Metadata.PhotoURL)
	assert.Empty(t, res[1].Metadata.PhotoURL)
	assert.Empty(t, res[2].Metadata.PhotoURL)
	assert.Nil(t, res[4].Metadata)
	assert.Equal(t, "https://signed/c", res[5].Metadata.PhotoURL)
}
