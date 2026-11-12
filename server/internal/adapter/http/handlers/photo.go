package handlers

import (
	"context"

	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/internal/adapter/http/httpmodel"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/log"
	"github.com/sourcegraph/conc/pool"
)

const maxSignConcurrency = 10

// meResponse signs the photo and fails when signing fails, as the GraphQL me
// query does.
func meResponse(ctx context.Context, photos adapter.PhotoSigner, u *user.User) (*httpmodel.MeResponse, error) {
	res := httpmodel.NewMeResponse(u)
	if res == nil {
		return nil, nil
	}
	signed, err := photos.Sign(ctx, res.Metadata.PhotoURL)
	if err != nil {
		return nil, err
	}
	res.Metadata.PhotoURL = signed
	return res, nil
}

// signWorkspacePhotos logs a signing failure and leaves that photo_url empty,
// as GraphQL's ToWorkspace does.
func signWorkspacePhotos(ctx context.Context, photos adapter.PhotoSigner, res []*httpmodel.WorkspaceResponse) {
	p := pool.New().WithMaxGoroutines(maxSignConcurrency)
	for _, r := range res {
		if r == nil || r.Metadata == nil || r.Metadata.PhotoURL == "" {
			continue
		}
		m := r.Metadata
		p.Go(func() {
			signed, err := photos.Sign(ctx, m.PhotoURL)
			if err != nil {
				log.Errorfc(ctx, "rest: failed to sign photo url: %v", err)
			}
			m.PhotoURL = signed
		})
	}
	p.Wait()
}
