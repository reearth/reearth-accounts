package mail

import "github.com/reearth/reearth-accounts/server/pkg/id"

type Kind string

const (
	KindVerification       Kind = "verification"
	KindPasswordReset      Kind = "password_reset"
	KindVerificationResend Kind = "verification_resend"
)

// The user ID is the only recipient field that may be logged.
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

// Message is a rendered mail. Only Render builds one, so a message always has
// a known kind and both bodies.
type Message struct {
	kind    Kind
	subject string
	text    string
	html    string
}

func (m Message) Kind() Kind      { return m.kind }
func (m Message) Subject() string { return m.subject }
func (m Message) Text() string    { return m.text }
func (m Message) HTML() string    { return m.html }

type Envelope struct {
	To      Recipient
	Message Message
}

// SendError hides the provider error text, because provider errors can quote
// the recipient address; the cause stays reachable through Unwrap.
type SendError struct {
	Err error
}

func (e *SendError) Error() string { return "mail: send failed" }

func (e *SendError) Unwrap() error { return e.Err }
