package adapter

import (
	"context"
	"errors"
	"testing"

	gatewaymock "github.com/reearth/reearth-accounts/server/internal/usecase/gateway/mock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestSignPhotoURL(t *testing.T) {
	ctx := context.Background()

	t.Run("empty path is not signed", func(t *testing.T) {
		got, err := SignPhotoURL(ctx, gatewaymock.NewMockStorage(gomock.NewController(t)), "")
		assert.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("signed", func(t *testing.T) {
		s := gatewaymock.NewMockStorage(gomock.NewController(t))
		s.EXPECT().GetSignedURL(gomock.Any(), "users/a.png").Return("https://signed/a", nil)
		got, err := SignPhotoURL(ctx, s, "users/a.png")
		assert.NoError(t, err)
		assert.Equal(t, "https://signed/a", got)
	})

	t.Run("failure returns no URL", func(t *testing.T) {
		s := gatewaymock.NewMockStorage(gomock.NewController(t))
		s.EXPECT().GetSignedURL(gomock.Any(), "users/a.png").Return("users/a.png", errors.New("gcs down"))
		got, err := SignPhotoURL(ctx, s, "users/a.png")
		assert.Error(t, err)
		assert.Empty(t, got)
	})
}
