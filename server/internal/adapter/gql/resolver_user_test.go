package gql

import (
	"context"
	"errors"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/internal/adapter/gql/gqlmodel"
	gatewaymock "github.com/reearth/reearth-accounts/server/internal/usecase/gateway/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestUserMetadataPhotoURL_SigningErrorReturnsNoPhoto(t *testing.T) {
	s := gatewaymock.NewMockStorage(gomock.NewController(t))
	s.EXPECT().GetSignedURL(gomock.Any(), "users/a.png").Return("users/a.png", errors.New("gcs down"))
	photos, err := adapter.NewPhotoSigner(s)
	require.NoError(t, err)

	r := &Resolver{Photos: photos}
	got, err := r.UserMetadata().PhotoURL(context.Background(), &gqlmodel.UserMetadata{PhotoURL: "users/a.png"})
	assert.NoError(t, err)
	assert.Empty(t, got)
}
