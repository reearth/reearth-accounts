//go:generate go run go.uber.org/mock/mockgen@v0.5.1 -source $GOFILE -destination ./mock/mock_storage.go -package mock -mock_names Service=MockStorageGateway
package gateway

import (
	"context"

	"github.com/reearth/reearthx/asset/domain/file"
)

const (
	GcsCMSBasePath       string = "cms"
	GcsUserBasePath      string = "users"
	GcsWorkspaceBasePath string = "workspaces"
)

type Storage interface {
	Delete(ctx context.Context, name string) error
	Upload(ctx context.Context, name string, data *file.File) error
	GetSignedURL(ctx context.Context, name string) (string, error)
}
