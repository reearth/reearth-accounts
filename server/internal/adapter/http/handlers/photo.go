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

// signPhotoURL returns what GraphQL's ToWorkspace returns for a stored photo:
// the signed URL, or empty when signing fails.
func signPhotoURL(ctx context.Context, storage gateway.Storage, stored string) string {
	if stored == "" || storage == nil {
		return stored
	}
	signed, err := storage.GetSignedURL(ctx, stored)
	if err != nil {
		log.Errorfc(ctx, "rest: failed to sign photo url: %v", err)
	}
	return signed
}

func meResponse(ctx context.Context, storage gateway.Storage, u *user.User) *httpmodel.MeResponse {
	res := httpmodel.NewMeResponse(u)
	if res != nil {
		res.Metadata.PhotoURL = signPhotoURL(ctx, storage, res.Metadata.PhotoURL)
	}
	return res
}

func signWorkspacePhotos(ctx context.Context, storage gateway.Storage, res []*httpmodel.WorkspaceResponse) {
	p := pool.New().WithMaxGoroutines(maxSignConcurrency)
	for _, r := range res {
		if r == nil || r.Metadata == nil || r.Metadata.PhotoURL == "" {
			continue
		}
		p.Go(func() {
			r.Metadata.PhotoURL = signPhotoURL(ctx, storage, r.Metadata.PhotoURL)
		})
	}
	p.Wait()
}
