package adapter

import (
	"context"
	"errors"

	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/server/pkg/applog"
	"github.com/sourcegraph/conc/pool"
)

const maxSignConcurrency = 10

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

// SignAll replaces each stored photo in place with its signed URL. A failure
// is logged and leaves that photo empty rather than failing the response.
func (s PhotoSigner) SignAll(ctx context.Context, photos []*string) {
	p := pool.New().WithMaxGoroutines(maxSignConcurrency)
	for _, photo := range photos {
		if photo == nil || *photo == "" {
			continue
		}
		p.Go(func() {
			signed, err := s.Sign(ctx, *photo)
			if err != nil {
				applog.Error(ctx, "failed to sign photo url", applog.Err(err))
			}
			*photo = signed
		})
	}
	p.Wait()
}
