package gqlmodel

import (
	"context"
	"sync"

	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearthx/log"
	"github.com/sourcegraph/conc/pool"
)

// signUserPhotoURLsConcurrency caps parallel GetSignedURL calls per request.
const signUserPhotoURLsConcurrency = 10

// SignUserPhotoURLs replaces each user's stored photo object path with a
// signed URL. Each distinct path is signed once. Signing is best-effort: a
// failure is logged and the photo URL is cleared, matching ToWorkspace, so a
// bare storage path is never returned to clients.
func SignUserPhotoURLs(ctx context.Context, users []*User, storage gateway.Storage) {
	if storage == nil || len(users) == 0 {
		return
	}

	unique := make(map[string]struct{})
	for _, u := range users {
		if u != nil && u.Metadata != nil && u.Metadata.PhotoURL != "" {
			unique[u.Metadata.PhotoURL] = struct{}{}
		}
	}
	if len(unique) == 0 {
		return
	}

	var mu sync.Mutex
	signed := make(map[string]string, len(unique))
	p := pool.New().WithMaxGoroutines(signUserPhotoURLsConcurrency)
	for path := range unique {
		p.Go(func() {
			url, err := storage.GetSignedURL(ctx, path)
			if err != nil {
				// Do not log the raw path: it contains the user ID.
				log.Errorfc(ctx, "[SignUserPhotoURLs] failed to get signed url: %v", err)
				return
			}
			mu.Lock()
			signed[path] = url
			mu.Unlock()
		})
	}
	p.Wait()

	for _, u := range users {
		if u != nil && u.Metadata != nil && u.Metadata.PhotoURL != "" {
			u.Metadata.PhotoURL = signed[u.Metadata.PhotoURL]
		}
	}
}
