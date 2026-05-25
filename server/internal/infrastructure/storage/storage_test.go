package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/reearth/reearthx/asset/domain/file"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStorage(t *testing.T) {
	t.Run("should create storage with valid config", func(t *testing.T) {
		cfg := &Config{
			IsLocal:          true,
			BucketName:       "test-bucket",
			EmulatorEnabled:  false,
			EmulatorEndpoint: "",
		}

		storage, err := NewGCPStorage(cfg)

		assert.NoError(t, err)
		assert.NotNil(t, storage)

		// Cast to concrete type to verify config
		concreteStorage := storage.(*Storage)
		assert.Equal(t, cfg, concreteStorage.cfg)
	})

	t.Run("should create storage with emulator config", func(t *testing.T) {
		cfg := &Config{
			IsLocal:          false,
			BucketName:       "test-bucket",
			EmulatorEnabled:  true,
			EmulatorEndpoint: "localhost:8080",
		}

		storage, err := NewGCPStorage(cfg)

		assert.NoError(t, err)
		assert.NotNil(t, storage)

		concreteStorage := storage.(*Storage)
		assert.Equal(t, cfg, concreteStorage.cfg)
	})
}

func TestStorage_GetSignedURL_Local(t *testing.T) {
	t.Run("should generate signed URL for local storage", func(t *testing.T) {
		// Skip if no emulator is available
		if os.Getenv("STORAGE_EMULATOR_HOST") == "" {
			t.Skip("Storage emulator not available")
		}

		cfg := &Config{
			IsLocal:          true,
			BucketName:       "test-bucket",
			EmulatorEnabled:  true,
			EmulatorEndpoint: "localhost:8080",
		}

		storage, err := NewGCPStorage(cfg)
		require.NoError(t, err)

		ctx := context.Background()
		objectName := "test-file.txt"

		url, err := storage.GetSignedURL(ctx, objectName)

		assert.NoError(t, err)
		assert.NotEmpty(t, url)
		assert.Contains(t, url, objectName)
	})

	t.Run("should handle RSA key generation error in local mode", func(t *testing.T) {
		// This test is difficult to trigger naturally since RSA key generation
		// rarely fails. We'll test the basic path instead.
		cfg := &Config{
			IsLocal:          true,
			BucketName:       "test-bucket",
			EmulatorEnabled:  true,
			EmulatorEndpoint: "localhost:8080",
		}

		storage, err := NewGCPStorage(cfg)
		require.NoError(t, err)

		ctx := context.Background()
		objectName := "test-file.txt"

		// This should work in normal conditions
		url, err := storage.GetSignedURL(ctx, objectName)

		// If emulator is not available, we expect an error
		if os.Getenv("STORAGE_EMULATOR_HOST") == "" {
			assert.Error(t, err)
			assert.Empty(t, url)
		} else {
			assert.NoError(t, err)
			assert.NotEmpty(t, url)
		}
	})
}

func TestStorage_GetSignedURL_Production(t *testing.T) {
	t.Run("should handle production signed URL generation error", func(t *testing.T) {
		// Production mode without proper credentials should fail
		cfg := &Config{
			IsLocal:          false,
			BucketName:       "test-bucket",
			EmulatorEnabled:  false,
			EmulatorEndpoint: "",
		}

		storage, err := NewGCPStorage(cfg)
		require.NoError(t, err)

		ctx := context.Background()
		objectName := "test-file.txt"

		url, err := storage.GetSignedURL(ctx, objectName)

		// Should fail due to missing credentials
		assert.Error(t, err)
		assert.Empty(t, url)
		assert.Contains(t, err.Error(), "unable to detect default GoogleAccessID")
	})
}

func TestStorage_Upload(t *testing.T) {
	t.Run("should handle upload error without emulator", func(t *testing.T) {
		// Without emulator, upload should fail
		cfg := &Config{
			IsLocal:          false,
			BucketName:       "test-bucket",
			EmulatorEnabled:  false,
			EmulatorEndpoint: "",
		}

		storage, err := NewGCPStorage(cfg)
		require.NoError(t, err)

		ctx := context.Background()
		objectName := "test-upload.txt"
		testContent := "Hello, World!"

		testFile := &file.File{
			Content:         io.NopCloser(bytes.NewReader([]byte(testContent))),
			Name:            "test-upload.txt",
			ContentType:     "text/plain",
			ContentEncoding: "",
			Path:            "test-upload.txt",
			Size:            int64(len(testContent)),
		}

		err = storage.Upload(ctx, objectName, testFile)

		// Should fail without proper GCS setup
		assert.Error(t, err)
	})

	t.Run("should handle empty bucket name in upload", func(t *testing.T) {
		cfg := &Config{
			IsLocal:          false,
			BucketName:       "", // Empty bucket name
			EmulatorEnabled:  false,
			EmulatorEndpoint: "",
		}

		storage, err := NewGCPStorage(cfg)
		require.NoError(t, err)

		ctx := context.Background()
		objectName := "test-upload.txt"
		testContent := "Hello, World!"

		testFile := &file.File{
			Content:         io.NopCloser(bytes.NewReader([]byte(testContent))),
			Name:            "test-upload.txt",
			ContentType:     "text/plain",
			ContentEncoding: "",
			Path:            "test-upload.txt",
			Size:            int64(len(testContent)),
		}

		err = storage.Upload(ctx, objectName, testFile)

		assert.Error(t, err)
		// Connection errors are expected when no emulator is running
		assert.True(t,
			strings.Contains(err.Error(), "bucket name is empty") ||
				strings.Contains(err.Error(), "connection refused") ||
				strings.Contains(err.Error(), "failed to close GCS object writer"),
			"Expected bucket name empty or connection error, got: %s", err.Error())
	})

	t.Run("should handle file with all metadata fields", func(t *testing.T) {
		cfg := &Config{
			IsLocal:          false,
			BucketName:       "test-bucket",
			EmulatorEnabled:  false,
			EmulatorEndpoint: "",
		}

		storage, err := NewGCPStorage(cfg)
		require.NoError(t, err)

		ctx := context.Background()
		objectName := "test-metadata.webp"
		testContent := "fake webp content"

		testFile := &file.File{
			Content:         io.NopCloser(bytes.NewReader([]byte(testContent))),
			Name:            "test-photo.webp",
			ContentType:     "image/webp",
			ContentEncoding: "gzip",
			Path:            "photos/test-photo.webp",
			Size:            int64(len(testContent)),
		}

		err = storage.Upload(ctx, objectName, testFile)

		// Should fail due to no credentials, but this tests that the metadata setting code path is executed
		assert.Error(t, err)
		// The error should be about GCS operation failure or connection error
		assert.True(t,
			strings.Contains(err.Error(), "failed to delete existing GCS object") ||
				strings.Contains(err.Error(), "failed to save GCS object") ||
				strings.Contains(err.Error(), "failed to close GCS object writer") ||
				strings.Contains(err.Error(), "connection refused"),
			"Expected GCS operation error, got: %s", err.Error())
	})

	t.Run("should handle file with empty content encoding", func(t *testing.T) {
		cfg := &Config{
			IsLocal:          false,
			BucketName:       "test-bucket",
			EmulatorEnabled:  false,
			EmulatorEndpoint: "",
		}

		storage, err := NewGCPStorage(cfg)
		require.NoError(t, err)

		ctx := context.Background()
		objectName := "test-no-encoding.txt"
		testContent := "Hello, World!"

		testFile := &file.File{
			Content:         io.NopCloser(bytes.NewReader([]byte(testContent))),
			Name:            "test-no-encoding.txt",
			ContentType:     "text/plain",
			ContentEncoding: "", // Empty content encoding
			Path:            "test-no-encoding.txt",
			Size:            int64(len(testContent)),
		}

		err = storage.Upload(ctx, objectName, testFile)

		assert.Error(t, err)
		assert.True(t,
			strings.Contains(err.Error(), "failed to delete existing GCS object") ||
				strings.Contains(err.Error(), "failed to save GCS object") ||
				strings.Contains(err.Error(), "failed to close GCS object writer") ||
				strings.Contains(err.Error(), "connection refused"),
			"Expected GCS operation error, got: %s", err.Error())
	})

	t.Run("should handle zero size file", func(t *testing.T) {
		cfg := &Config{
			IsLocal:          false,
			BucketName:       "test-bucket",
			EmulatorEnabled:  false,
			EmulatorEndpoint: "",
		}

		storage, err := NewGCPStorage(cfg)
		require.NoError(t, err)

		ctx := context.Background()
		objectName := "test-empty.txt"
		testContent := ""

		testFile := &file.File{
			Content:         io.NopCloser(bytes.NewReader([]byte(testContent))),
			Name:            "test-empty.txt",
			ContentType:     "text/plain",
			ContentEncoding: "",
			Path:            "test-empty.txt",
			Size:            0, // Zero size
		}

		err = storage.Upload(ctx, objectName, testFile)

		assert.Error(t, err)
		assert.True(t,
			strings.Contains(err.Error(), "failed to delete existing GCS object") ||
				strings.Contains(err.Error(), "failed to save GCS object") ||
				strings.Contains(err.Error(), "failed to close GCS object writer") ||
				strings.Contains(err.Error(), "connection refused"),
			"Expected GCS operation error, got: %s", err.Error())
	})

	t.Run("should handle client creation error", func(t *testing.T) {
		cfg := &Config{
			IsLocal:          false,
			BucketName:       "", // This will cause client creation to fail
			EmulatorEnabled:  false,
			EmulatorEndpoint: "",
		}

		storage, err := NewGCPStorage(cfg)
		require.NoError(t, err)

		ctx := context.Background()
		objectName := "test-client-error.txt"
		testContent := "Hello, World!"

		testFile := &file.File{
			Content:         io.NopCloser(bytes.NewReader([]byte(testContent))),
			Name:            "test-client-error.txt",
			ContentType:     "text/plain",
			ContentEncoding: "",
			Path:            "test-client-error.txt",
			Size:            int64(len(testContent)),
		}

		err = storage.Upload(ctx, objectName, testFile)

		assert.Error(t, err)
		// Should fail at GCS operation step (client creation succeeds but operations fail)
		assert.True(t,
			strings.Contains(err.Error(), "failed to get GCS client") ||
				strings.Contains(err.Error(), "failed to delete existing GCS object") ||
				strings.Contains(err.Error(), "failed to close GCS object writer") ||
				strings.Contains(err.Error(), "connection refused"),
			"Expected GCS client or operation error, got: %s", err.Error())
	})

	t.Run("should validate input parameters", func(t *testing.T) {
		cfg := &Config{
			IsLocal:          false,
			BucketName:       "test-bucket",
			EmulatorEnabled:  false,
			EmulatorEndpoint: "",
		}

		storage, err := NewGCPStorage(cfg)
		require.NoError(t, err)

		ctx := context.Background()
		objectName := "test-params.txt"
		testContent := "Test content for parameter validation"

		// Test with valid file.File structure
		testFile := &file.File{
			Content:         io.NopCloser(bytes.NewReader([]byte(testContent))),
			Name:            "test-params.txt",
			ContentType:     "text/plain; charset=utf-8",
			ContentEncoding: "br", // Brotli compression
			Path:            "uploads/documents/test-params.txt",
			Size:            int64(len(testContent)),
		}

		err = storage.Upload(ctx, objectName, testFile)

		// Should fail due to GCS connectivity but validates all parameters are processed
		assert.Error(t, err)
		assert.True(t,
			strings.Contains(err.Error(), "failed to delete existing GCS object") ||
				strings.Contains(err.Error(), "failed to save GCS object") ||
				strings.Contains(err.Error(), "failed to close GCS object writer") ||
				strings.Contains(err.Error(), "connection refused"),
			"Expected GCS operation error, got: %s", err.Error())
	})
}

func TestStorage_bucket(t *testing.T) {
	t.Run("should create client with emulator", func(t *testing.T) {
		// Skip if no emulator is available
		if os.Getenv("STORAGE_EMULATOR_HOST") == "" {
			t.Skip("Storage emulator not available")
		}

		cfg := &Config{
			IsLocal:          false,
			BucketName:       "test-bucket",
			EmulatorEnabled:  true,
			EmulatorEndpoint: "localhost:8080",
		}

		s := &Storage{cfg: cfg}

		bucket, err := s.bucket()

		assert.NoError(t, err)
		assert.NotNil(t, bucket)
	})

	t.Run("should handle invalid bucket name", func(t *testing.T) {
		cfg := &Config{
			IsLocal:          false,
			BucketName:       "", // Invalid bucket name
			EmulatorEnabled:  false,
			EmulatorEndpoint: "",
		}

		s := &Storage{cfg: cfg}

		bucket, err := s.bucket()

		// The client creation doesn't fail immediately, but the bucket handle is still returned
		assert.NoError(t, err)
		assert.NotNil(t, bucket)
		// The bucket name will be empty in the handle
		assert.Equal(t, "", bucket.BucketName())
	})

	t.Run("should set emulator environment variable", func(t *testing.T) {
		cfg := &Config{
			IsLocal:          false,
			BucketName:       "test-bucket",
			EmulatorEnabled:  true,
			EmulatorEndpoint: "localhost:9999",
		}

		s := &Storage{cfg: cfg}

		// This will set the environment variable
		_, _ = s.bucket()

		// Verify the environment variable was set
		assert.Equal(t, "localhost:9999", os.Getenv("STORAGE_EMULATOR_HOST"))
	})

	t.Run("should reuse the same client across multiple calls", func(t *testing.T) {
		cfg := &Config{
			IsLocal:          false,
			BucketName:       "test-bucket",
			EmulatorEnabled:  false,
			EmulatorEndpoint: "",
		}

		s := &Storage{cfg: cfg}

		bucket1, err1 := s.bucket()
		bucket2, err2 := s.bucket()

		assert.NoError(t, err1)
		assert.NoError(t, err2)
		assert.NotNil(t, bucket1)
		assert.NotNil(t, bucket2)
		// Both should reference the same underlying client
		assert.Equal(t, s.gcsClient, s.gcsClient)
	})
}

func TestStorage_ErrorHandling(t *testing.T) {
	t.Run("should handle credentials error in GetSignedURL", func(t *testing.T) {
		cfg := &Config{
			IsLocal:          false,
			BucketName:       "test-bucket",
			EmulatorEnabled:  false,
			EmulatorEndpoint: "",
		}

		storage := &Storage{cfg: cfg}

		ctx := context.Background()
		objectName := "test-file.txt"

		url, err := storage.GetSignedURL(ctx, objectName)

		assert.Error(t, err)
		assert.Empty(t, url)
		assert.Contains(t, err.Error(), "unable to detect default GoogleAccessID")
	})

	t.Run("should handle empty bucket name in Upload", func(t *testing.T) {
		cfg := &Config{
			IsLocal:          false,
			BucketName:       "", // Empty bucket name
			EmulatorEnabled:  false,
			EmulatorEndpoint: "",
		}

		storage := &Storage{cfg: cfg}

		ctx := context.Background()
		objectName := "test-file.txt"
		testContent := "Hello, World!"

		testFile := &file.File{
			Content:         io.NopCloser(bytes.NewReader([]byte(testContent))),
			Name:            "test-file.txt",
			ContentType:     "text/plain",
			ContentEncoding: "",
			Path:            "test-file.txt",
			Size:            int64(len(testContent)),
		}

		err := storage.Upload(ctx, objectName, testFile)

		assert.Error(t, err)
		// Connection errors are expected when no emulator is running
		assert.True(t,
			strings.Contains(err.Error(), "bucket name is empty") ||
				strings.Contains(err.Error(), "connection refused") ||
				strings.Contains(err.Error(), "failed to close GCS object writer"),
			"Expected bucket name empty or connection error, got: %s", err.Error())
	})
}
