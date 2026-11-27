package otel

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateResource_GCPDetectorOnGCE(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Metadata-Flavor", "Google")
		switch {
		case strings.HasSuffix(r.URL.Path, "/project/project-id"):
			_, _ = w.Write([]byte("test-project"))
		case strings.HasSuffix(r.URL.Path, "/instance/id"):
			_, _ = w.Write([]byte("123"))
		case strings.HasSuffix(r.URL.Path, "/instance/zone"):
			_, _ = w.Write([]byte("projects/1/zones/us-central1-a"))
		case strings.HasSuffix(r.URL.Path, "/instance/region"):
			_, _ = w.Write([]byte("projects/1/regions/us-central1"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("GCE_METADATA_HOST", strings.TrimPrefix(srv.URL, "http://"))
	t.Setenv("K_SERVICE", "accounts")
	t.Setenv("K_REVISION", "accounts-1")
	t.Setenv("K_CONFIGURATION", "accounts")

	res, err := createResource(context.Background(), &Config{ExporterType: ExporterTypeGCP, ServiceName: "accounts"})
	require.NoError(t, err)

	attrs := map[string]string{}
	for _, kv := range res.Attributes() {
		attrs[string(kv.Key)] = kv.Value.String()
	}
	assert.Equal(t, "gcp", attrs["cloud.provider"])
	assert.Equal(t, "accounts", attrs["service.name"])
}
