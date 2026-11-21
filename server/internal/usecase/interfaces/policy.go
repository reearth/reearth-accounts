package interfaces

import (
	"context"
	"io"

	"github.com/reearth/reearth-accounts/server/pkg/legal"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
)

type Policy interface {
	Accept(ctx context.Context, refs []legal.RefInput, client legal.Client, op *workspace.Operator) (*user.User, error)
	Overview(ctx context.Context, op *workspace.Operator) (legal.Overview, error)
	// ReceiveWebhook returns legal.ErrNotConfigured, legal.ErrPayloadTooLarge,
	// legal.ErrBadSignature or a *legal.PayloadError when it saves nothing.
	ReceiveWebhook(ctx context.Context, body io.Reader, signature string) (legal.ApplyResult, error)
}
