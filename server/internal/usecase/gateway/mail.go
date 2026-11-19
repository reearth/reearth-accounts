package gateway

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/mail"
)

// Mail is the only way accounts sends mail.
type Mail interface {
	// Deliverable reports whether a provider is configured.
	Deliverable() bool
	// Send delivers one message to one recipient now. With no provider it
	// drops the message and returns nil. A provider failure is a *mail.SendError.
	Send(ctx context.Context, e mail.Envelope) error
}
