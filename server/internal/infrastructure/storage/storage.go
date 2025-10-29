package storage

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"os"
	"time"

	"cloud.google.com/go/storage"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearthx/asset/domain/file"
	"github.com/reearth/reearthx/log"
	"google.golang.org/api/option"
)

type Config struct {
	IsLocal          bool
	BucketName       string
	EmulatorEnabled  bool
	EmulatorEndpoint string
}

type Storage struct {
	cfg *Config
}

func NewGCPStorage(cfg *Config) (gateway.Storage, error) {
	return &Storage{
		cfg: cfg,
	}, nil
}

func (s *Storage) GetSignedURL(ctx context.Context, name string) (string, error) {
	c, err := s.client(ctx)
	if err != nil {
		return "", err
	}

	// If the storage is local, we generate a signed URL with a temporary RSA key.
	if s.cfg.IsLocal {
		key, kErr := rsa.GenerateKey(rand.Reader, 2048)
		if kErr != nil {
			return "", kErr
		}

		pri := pem.EncodeToMemory(
			&pem.Block{
				Type:  "RSA PRIVATE KEY",
				Bytes: x509.MarshalPKCS1PrivateKey(key),
			},
		)

		url, sErr := c.SignedURL(name, &storage.SignedURLOptions{
			Method:         "GET",
			Expires:        time.Now().Add(24 * time.Hour),
			GoogleAccessID: "default",
			PrivateKey:     pri,
		})
		if sErr != nil {
			return "", sErr
		}

		return url, nil
	}

	url, sErr := c.SignedURL(name, &storage.SignedURLOptions{
		Method:  "GET",
		Expires: time.Now().Add(24 * time.Hour),
	})
	if sErr != nil {
		return "", sErr
	}

	return url, nil
}

func (s *Storage) client(ctx context.Context) (*storage.BucketHandle, error) {
	var (
		opts   []option.ClientOption
		client *storage.Client
		err    error
	)

	// For local development & testing purposes.
	if s.cfg.EmulatorEnabled {
		_ = os.Setenv("STORAGE_EMULATOR_HOST", s.cfg.EmulatorEndpoint)
	}

	if s.cfg.IsLocal {
		opts = append(opts, option.WithoutAuthentication())
	}

	client, err = storage.NewClient(ctx, opts...)
	if err != nil {
		return nil, err
	}

	// Close client when context is done - use context-aware cleanup
	go func() {
		<-ctx.Done()
		if cErr := client.Close(); cErr != nil {
			log.Errorf("failed to close GCS client: %v", cErr)
		}
	}()

	return client.Bucket(s.cfg.BucketName), nil
}

func (s *Storage) Delete(ctx context.Context, name string) error {
	c, err := s.client(ctx)
	if err != nil {
		return err
	}

	obj := c.Object(name)
	if err = obj.Delete(ctx); err != nil && !errors.Is(err, storage.ErrObjectNotExist) {
		return err
	}

	return nil
}

func (s *Storage) Upload(ctx context.Context, name string, data *file.File) error {
	c, err := s.client(ctx)
	if err != nil {
		return err
	}

	obj := c.Object(name)
	w := obj.NewWriter(ctx)
	w.CacheControl = "public, max-age=2592000, immutable"

	_, err = io.Copy(w, data.Content)
	if err != nil {
		return err
	}

	if err = w.Close(); err != nil {
		return err
	}

	return nil
}
