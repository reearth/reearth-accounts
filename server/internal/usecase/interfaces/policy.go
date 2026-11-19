package interfaces

import (
	"context"
	"io"

	"github.com/reearth/reearth-accounts/server/pkg/legal"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
)

type Policy interface {
	// Accept records the operator's acceptance of refs with the calling
	// client. It returns legal.ErrNotConfigured, ErrInvalidOperator for a
	// caller that is not a user, legal.ErrUnknownType or
	// legal.ErrNotAcceptable, and then records nothing.
	Accept(ctx context.Context, refs []legal.Ref, client legal.Client, op *workspace.Operator) (*user.User, error)
	// Overview is empty when policies are not configured.
	Overview(ctx context.Context, op *workspace.Operator) (legal.Overview, error)
	// ReceiveWebhook returns legal.ErrNotConfigured, legal.ErrPayloadTooLarge,
	// legal.ErrBadSignature or a *legal.PayloadError when it saves nothing.
	ReceiveWebhook(ctx context.Context, body io.Reader, signature string) (legal.ApplyResult, error)
}
