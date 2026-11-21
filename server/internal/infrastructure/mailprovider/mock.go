package mailprovider

import (
	"context"
	"sync"

	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/server/pkg/mail"
)

var _ gateway.Mail = (*Mock)(nil)

type Mock struct {
	Err        error
	NoProvider bool
	envelopes  []mail.Envelope
	lock       sync.Mutex
}

func (m *Mock) Deliverable() bool { return !m.NoProvider }

func (m *Mock) Envelopes() []mail.Envelope {
	m.lock.Lock()
	defer m.lock.Unlock()
	return append([]mail.Envelope{}, m.envelopes...)
}

func (m *Mock) Send(_ context.Context, e mail.Envelope) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.envelopes = append(m.envelopes, e)
	return m.Err
}
