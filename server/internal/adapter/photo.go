package adapter

import (
	"context"

	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
)

// SignPhotoURL signs a stored photo path for both APIs. It returns "" for an
// empty path, and "" with the error when signing fails, so a caller that
// drops the error never exposes the stored path.
func SignPhotoURL(ctx context.Context, storage gateway.Storage, stored string) (string, error) {
	if stored == "" {
		return "", nil
	}
	signed, err := storage.GetSignedURL(ctx, stored)
	if err != nil {
		return "", err
	}
	return signed, nil
}
