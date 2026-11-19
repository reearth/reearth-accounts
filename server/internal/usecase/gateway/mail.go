package gateway

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/mail"
)

type Mail interface {
	// Deliverable reports whether a provider is configured. Queued mail
	// waits while it is false.
	Deliverable() bool
	Send(ctx context.Context, e mail.Envelope) error
}
