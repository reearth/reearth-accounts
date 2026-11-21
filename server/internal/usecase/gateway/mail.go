package gateway

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/mail"
)

type Mail interface {
	Deliverable() bool
	Send(ctx context.Context, e mail.Envelope) error
}
