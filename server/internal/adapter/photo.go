package adapter

import (
	"context"
	"errors"

	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
)

// PhotoSigner signs stored photo paths for both APIs. Build it with
// NewPhotoSigner.
type PhotoSigner struct {
	storage gateway.Storage
}

func NewPhotoSigner(storage gateway.Storage) (PhotoSigner, error) {
	if storage == nil {
		return PhotoSigner{}, errors.New("photo signer: storage is required")
	}
	return PhotoSigner{storage: storage}, nil
}

// Sign returns "" for an empty path, and "" with the error when signing
// fails, so a caller that drops the error never exposes the stored path.
func (s PhotoSigner) Sign(ctx context.Context, stored string) (string, error) {
	if stored == "" {
		return "", nil
	}
	signed, err := s.storage.GetSignedURL(ctx, stored)
	if err != nil {
		return "", err
	}
	return signed, nil
}
