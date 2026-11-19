// Package outbox holds queued mail. A record leaves sending only through a
// claim-guarded result or an expired claim, so a recipient gets each message
// at most once.
package outbox

import (
	"errors"
	"strings"
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/mail"
)

const ClaimLease = 10 * time.Minute

// Key is chosen by the feature that queues the record, so queuing the same
// mail twice finds the first record.
type Key string

type State string

const (
	StatePending State = "pending"
	StateSending State = "sending"
	StateSent    State = "sent"
	StateFailed  State = "failed"
	StateUnknown State = "unknown"
)

var States = []State{StatePending, StateSending, StateSent, StateFailed, StateUnknown}

func (s State) Ended() bool { return s == StateSent || s == StateFailed || s == StateUnknown }

type Reason string

const (
	ReasonTTL          Reason = "ttl_expired"
	ReasonMaxAttempts  Reason = "max_attempts"
	ReasonPermanent    Reason = "permanent_error"
	ReasonClaimExpired Reason = "claim_expired"
	ReasonUnknownKind  Reason = "unknown_kind"
	ReasonUserGone     Reason = "user_gone"
)

var Reasons = []Reason{ReasonTTL, ReasonMaxAttempts, ReasonPermanent, ReasonClaimExpired, ReasonUnknownKind, ReasonUserGone}

type Subject map[string]string

// Recipient is a user, or an address only for a person without an account.
// A user's address and language are read when the mail is sent.
type Recipient struct {
	address string
	user    id.UserID
}

func ToUser(u id.UserID) Recipient { return Recipient{user: u} }

func ToAddress(address string) Recipient {
	return Recipient{address: strings.ToLower(strings.TrimSpace(address))}
}

func (r Recipient) User() (id.UserID, bool) { return r.user, !r.user.IsNil() }

func (r Recipient) Address() (string, bool) { return r.address, r.user.IsNil() && r.address != "" }

func (r Recipient) valid() bool { return r.user.IsNil() != (r.address == "") }

type Claim struct {
	Token string
	Until time.Time
}

type Record struct {
	Attempts      int
	Claim         *Claim
	CreatedAt     time.Time
	EndedAt       *time.Time
	Key           Key
	Kind          mail.Kind
	NextAttemptAt time.Time
	Reason        Reason
	Recipient     Recipient
	State         State
	Subject       Subject
	UpdatedAt     time.Time
}

var (
	ErrEmptyKey         = errors.New("outbox: empty key")
	ErrEmptyKind        = errors.New("outbox: empty mail kind")
	ErrInvalidRecipient = errors.New("outbox: recipient must be exactly one of a user or an address")
)

func New(k Key, kind mail.Kind, s Subject, to Recipient, now time.Time) (Record, error) {
	switch {
	case k == "":
		return Record{}, ErrEmptyKey
	case kind == "":
		return Record{}, ErrEmptyKind
	case !to.valid():
		return Record{}, ErrInvalidRecipient
	}
	return Record{
		Key:           k,
		Kind:          kind,
		Subject:       s,
		Recipient:     to,
		State:         StatePending,
		NextAttemptAt: now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}
