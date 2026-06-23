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
	"sync"
	"time"

	"cloud.google.com/go/storage"
	lruexpirable "github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearthx/asset/domain/file"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/api/option"
)

const (
	signedURLExpiry    = 24 * time.Hour
	signedURLCacheTTL  = signedURLExpiry - 30*time.Minute
	signedURLCacheSize = 1024
)

type Config struct {
	IsLocal          bool
	BucketName       string
	EmulatorEnabled  bool
	EmulatorEndpoint string
}

type Storage struct {
	cfg       *Config
	once      sync.Once
	gcsClient *storage.Client
	initErr   error
	cache     *lruexpirable.LRU[string, string]
}

func NewGCPStorage(cfg *Config) (gateway.Storage, error) {
	return &Storage{
		cfg:   cfg,
		cache: lruexpirable.NewLRU[string, string](signedURLCacheSize, nil, signedURLCacheTTL),
	}, nil
}

func (s *Storage) GetSignedURL(ctx context.Context, name string) (string, error) {
	ctx, span := otel.Tracer("reearth-accounts").Start(ctx, "storage.GetSignedURL",
		trace.WithAttributes(
			attribute.String("storage.bucket", s.cfg.BucketName),
			attribute.Bool("storage.is_local", s.cfg.IsLocal),
		),
	)
	defer span.End()

	if cached, ok := s.cache.Get(name); ok {
		return cached, nil
	}

	c, err := s.bucket(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "bucket initialization failed")
		return "", err
	}

	var url string
	// If the storage is local, we generate a signed URL with a temporary RSA key.
	if s.cfg.IsLocal {
		key, kErr := rsa.GenerateKey(rand.Reader, 2048)
		if kErr != nil {
			span.RecordError(kErr)
			span.SetStatus(codes.Error, "rsa key generation failed")
			return "", kErr
		}

		pri := pem.EncodeToMemory(
			&pem.Block{
				Type:  "RSA PRIVATE KEY",
				Bytes: x509.MarshalPKCS1PrivateKey(key),
			},
		)

		url, err = c.SignedURL(name, &storage.SignedURLOptions{
			Method:         "GET",
			Expires:        time.Now().Add(signedURLExpiry),
			GoogleAccessID: "default",
			PrivateKey:     pri,
		})
	} else {
		url, err = c.SignedURL(name, &storage.SignedURLOptions{
			Method:  "GET",
			Expires: time.Now().Add(signedURLExpiry),
		})
	}

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "signed URL generation failed")
		return "", err
	}

	s.cache.Add(name, url)

	return url, nil
}

func (s *Storage) bucket(ctx context.Context) (*storage.BucketHandle, error) {
	s.once.Do(func() {
		_, span := otel.Tracer("reearth-accounts").Start(ctx, "storage.initGCSClient")
		defer span.End()

		var opts []option.ClientOption
		if s.cfg.EmulatorEnabled {
			_ = os.Setenv("STORAGE_EMULATOR_HOST", s.cfg.EmulatorEndpoint)
		}
		if s.cfg.IsLocal {
			opts = append(opts, option.WithoutAuthentication())
		}
		// Use a non-cancelable context so a canceled caller doesn't permanently latch initErr.
		s.gcsClient, s.initErr = storage.NewClient(context.WithoutCancel(ctx), opts...)
		if s.initErr != nil {
			span.RecordError(s.initErr)
			span.SetStatus(codes.Error, "GCS client initialization failed")
		}
	})
	if s.initErr != nil {
		return nil, s.initErr
	}
	return s.gcsClient.Bucket(s.cfg.BucketName), nil
}

func (s *Storage) Delete(ctx context.Context, name string) error {
	c, err := s.bucket(ctx)
	if err != nil {
		return err
	}

	obj := c.Object(name)
	if err = obj.Delete(ctx); err != nil && !errors.Is(err, storage.ErrObjectNotExist) {
		return err
	}

	s.cache.Remove(name)

	return nil
}

func (s *Storage) Upload(ctx context.Context, name string, data *file.File) error {
	c, err := s.bucket(ctx)
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

	s.cache.Remove(name)

	return nil
}
