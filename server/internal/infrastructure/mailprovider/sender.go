package mailprovider

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/server/pkg/applog"
	"github.com/reearth/reearth-accounts/server/pkg/mail"
	"github.com/reearth/reearthx/mailer"
)

func New(ctx context.Context, s Settings) (gateway.Mail, error) {
	if s.Mode != ModeProvider {
		return &sender{mode: s.Mode, out: os.Stdout}, nil
	}
	m := mailer.New(ctx, &s.provider)
	if m == nil {
		return nil, fmt.Errorf("REEARTH_ACCOUNTS_MAILER: %s failed to initialize", s.provider.Mailer)
	}
	return &sender{mode: ModeProvider, m: m}, nil
}

type sender struct {
	m    mailer.Mailer
	mode Mode
	out  io.Writer
}

func (s *sender) Send(ctx context.Context, e mail.Envelope) error {
	if s.mode != ModeProvider {
		s.drop(ctx, e)
		return nil
	}
	to := []mailer.Contact{{Email: e.To.Address(), Name: e.To.Name()}}
	if err := s.m.SendMail(ctx, to, e.Message.Subject(), e.Message.Text(), e.Message.HTML()); err != nil {
		applog.Warn(ctx, "mail send failed", logFields(e)...)
		return &mail.SendError{Err: err, Permanent: permanent(err)}
	}
	return nil
}

func (s *sender) Deliverable() bool { return s.mode == ModeProvider }

func (s *sender) drop(ctx context.Context, e mail.Envelope) {
	applog.Warn(ctx, "mail dropped: no provider configured", logFields(e)...)
	if s.mode == ModeDevPrint {
		_, _ = fmt.Fprintln(s.out, e.Message.Text())
	}
}

// permanent is the one place that tells a permanent provider error from a
// transient one. reearthx returns no typed errors yet, so every error is
// treated as transient.
func permanent(error) bool { return false }

func logFields(e mail.Envelope) []applog.Field {
	fields := []applog.Field{applog.MailKind(e.Message.Kind())}
	if uid := e.To.UserID(); uid != nil {
		fields = append(fields, applog.UserID(*uid))
	}
	return fields
}
