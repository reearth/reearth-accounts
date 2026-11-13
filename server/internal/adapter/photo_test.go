package adapter

import (
	"context"
	"errors"
	"testing"

	gatewaymock "github.com/reearth/reearth-accounts/server/internal/usecase/gateway/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNewPhotoSigner(t *testing.T) {
	_, err := NewPhotoSigner(nil)
	assert.EqualError(t, err, "photo signer: storage is required")

	_, err = NewPhotoSigner(gatewaymock.NewMockStorage(gomock.NewController(t)))
	assert.NoError(t, err)
}

func TestPhotoSigner_Sign(t *testing.T) {
	ctx := context.Background()
	signer := func(t *testing.T) (PhotoSigner, *gatewaymock.MockStorage) {
		s := gatewaymock.NewMockStorage(gomock.NewController(t))
		p, err := NewPhotoSigner(s)
		require.NoError(t, err)
		return p, s
	}

	t.Run("empty path is not signed", func(t *testing.T) {
		p, _ := signer(t)
		got, err := p.Sign(ctx, "")
		assert.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("signed", func(t *testing.T) {
		p, s := signer(t)
		s.EXPECT().GetSignedURL(gomock.Any(), "users/a.png").Return("https://signed/a", nil)
		got, err := p.Sign(ctx, "users/a.png")
		assert.NoError(t, err)
		assert.Equal(t, "https://signed/a", got)
	})

	t.Run("failure returns no URL", func(t *testing.T) {
		p, s := signer(t)
		s.EXPECT().GetSignedURL(gomock.Any(), "users/a.png").Return("users/a.png", errors.New("gcs down"))
		got, err := p.Sign(ctx, "users/a.png")
		assert.Error(t, err)
		assert.Empty(t, got)
	})
}
