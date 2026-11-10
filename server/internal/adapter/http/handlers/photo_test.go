package handlers

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/adapter/http/httpmodel"
	gatewaymock "github.com/reearth/reearth-accounts/server/internal/usecase/gateway/mock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestSignWorkspacePhotos_StopsWhenContextIsDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res := []*httpmodel.WorkspaceResponse{
		{Metadata: &httpmodel.WorkspaceMetadataResponse{PhotoURL: "workspaces/a.png"}},
		{Metadata: &httpmodel.WorkspaceMetadataResponse{PhotoURL: "workspaces/b.png"}},
	}

	signWorkspacePhotos(ctx, gatewaymock.NewMockStorage(gomock.NewController(t)), res)

	assert.Empty(t, res[0].Metadata.PhotoURL)
	assert.Empty(t, res[1].Metadata.PhotoURL)
}
