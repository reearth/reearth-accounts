// Package mail is the vocabulary accounts uses to send mail: what kind of
// mail it is, the one person it goes to, and the rendered message.
package mail

import "github.com/reearth/reearth-accounts/server/pkg/id"

type Kind string

const (
	KindVerification       Kind = "verification"
	KindPasswordReset      Kind = "password_reset"
	KindVerificationResend Kind = "verification_resend"
)

// Recipient is exactly one person, so a send can never address two
// (ADR-0004 decision 12). The user ID is the only recipient field that may
// be logged.
type Recipient struct {
	userID  *id.UserID
	address string
	name    string
}

func ToUser(u id.UserID, address, name string) Recipient {
	return Recipient{userID: &u, address: address, name: name}
}

func (r Recipient) UserID() *id.UserID { return r.userID }
func (r Recipient) Address() string    { return r.address }
func (r Recipient) Name() string       { return r.name }

type Message struct {
	kind    Kind
	subject string
	text    string
	html    string
}

// Raw builds a message from bodies the caller already rendered.
func Raw(kind Kind, subject, text, html string) Message {
	return Message{kind: kind, subject: subject, text: text, html: html}
}

func (m Message) Kind() Kind      { return m.kind }
func (m Message) Subject() string { return m.subject }
func (m Message) Text() string    { return m.text }
func (m Message) HTML() string    { return m.html }

type Envelope struct {
	To      Recipient
	Message Message
}

// SendError is a provider failure. Its message names only the code, because
// provider errors can quote the recipient address; the cause stays reachable
// through Unwrap.
type SendError struct {
	Permanent bool
	Code      string
	Err       error
}

func (e *SendError) Error() string {
	if e.Code == "" {
		return "mail: send failed"
	}
	return "mail: send failed: " + e.Code
}

func (e *SendError) Unwrap() error { return e.Err }
