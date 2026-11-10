package handlers

import (
	"context"

	"github.com/reearth/reearth-accounts/server/internal/adapter/http/httpmodel"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/log"
	"github.com/sourcegraph/conc/pool"
)

const maxSignConcurrency = 10

func signPhotoURL(ctx context.Context, storage gateway.Storage, stored string) (string, error) {
	if stored == "" || storage == nil {
		return stored, nil
	}
	return storage.GetSignedURL(ctx, stored)
}

// meResponse signs the photo and fails when signing fails, as the GraphQL me
// query does.
func meResponse(ctx context.Context, storage gateway.Storage, u *user.User) (*httpmodel.MeResponse, error) {
	res := httpmodel.NewMeResponse(u)
	if res == nil {
		return nil, nil
	}
	signed, err := signPhotoURL(ctx, storage, res.Metadata.PhotoURL)
	if err != nil {
		return nil, err
	}
	res.Metadata.PhotoURL = signed
	return res, nil
}

// signWorkspacePhotos logs a signing failure and leaves that photo_url empty,
// as GraphQL's ToWorkspace does.

func signWorkspacePhotos(ctx context.Context, storage gateway.Storage, res []*httpmodel.WorkspaceResponse) {
	p := pool.New().WithMaxGoroutines(maxSignConcurrency)
	for _, r := range res {
		if r == nil || r.Metadata == nil || r.Metadata.PhotoURL == "" {
			continue
		}
		p.Go(func() {
			signed, err := signPhotoURL(ctx, storage, r.Metadata.PhotoURL)
			if err != nil {
				log.Errorfc(ctx, "rest: failed to sign photo url: %v", err)
			}
			r.Metadata.PhotoURL = signed
		})
	}
	p.Wait()
}
