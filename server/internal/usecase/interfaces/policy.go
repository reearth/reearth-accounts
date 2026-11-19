package interfaces

import (
	"context"
	"io"

	"github.com/reearth/reearth-accounts/server/pkg/legal"
)

type Policy interface {
	// ReceiveWebhook returns legal.ErrNotConfigured, legal.ErrPayloadTooLarge,
	// legal.ErrBadSignature or a *legal.PayloadError when it saves nothing.
	ReceiveWebhook(ctx context.Context, body io.Reader, signature string) (legal.ApplyResult, error)
}
