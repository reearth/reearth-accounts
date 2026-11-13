package gqlmodel

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/reearth/reearthx/asset/domain/file"
	"github.com/stretchr/testify/assert"
)

type fakeStorage struct {
	mu    sync.Mutex
	calls map[string]int
	fail  map[string]bool
}

func (f *fakeStorage) Delete(context.Context, string) error             { return nil }
func (f *fakeStorage) Upload(context.Context, string, *file.File) error { return nil }

func (f *fakeStorage) GetSignedURL(_ context.Context, name string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.calls == nil {
		f.calls = map[string]int{}
	}
	f.calls[name]++
	if f.fail[name] {
		return "", errors.New("sign failed")
	}
	return "https://signed.example/" + name, nil
}

func userWithPhoto(photo string) *User {
	return &User{Metadata: &UserMetadata{PhotoURL: photo}}
}

func TestSignUserPhotoURLs(t *testing.T) {
	ctx := context.Background()

	t.Run("signs each distinct path once and keeps empty photos empty", func(t *testing.T) {
		s := &fakeStorage{}
		users := []*User{
			userWithPhoto("users/a/1.webp"),
			userWithPhoto("users/a/1.webp"),
			userWithPhoto("users/b/2.webp"),
			userWithPhoto(""),
			{},
			nil,
		}

		SignUserPhotoURLs(ctx, users, s)

		assert.Equal(t, "https://signed.example/users/a/1.webp", users[0].Metadata.PhotoURL)
		assert.Equal(t, "https://signed.example/users/a/1.webp", users[1].Metadata.PhotoURL)
		assert.Equal(t, "https://signed.example/users/b/2.webp", users[2].Metadata.PhotoURL)
		assert.Equal(t, "", users[3].Metadata.PhotoURL)
		assert.Nil(t, users[4].Metadata)
		assert.Equal(t, map[string]int{"users/a/1.webp": 1, "users/b/2.webp": 1}, s.calls)
	})

	t.Run("clears the photo when signing fails instead of leaking the bare path", func(t *testing.T) {
		s := &fakeStorage{fail: map[string]bool{"users/bad.webp": true}}
		users := []*User{userWithPhoto("users/bad.webp"), userWithPhoto("users/ok.webp")}

		SignUserPhotoURLs(ctx, users, s)

		assert.Equal(t, "", users[0].Metadata.PhotoURL)
		assert.Equal(t, "https://signed.example/users/ok.webp", users[1].Metadata.PhotoURL)
	})

	t.Run("nil storage leaves users untouched", func(t *testing.T) {
		users := []*User{userWithPhoto("users/a/1.webp")}

		SignUserPhotoURLs(ctx, users, nil)

		assert.Equal(t, "users/a/1.webp", users[0].Metadata.PhotoURL)
	})
}
