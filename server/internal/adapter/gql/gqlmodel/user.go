package gqlmodel

// User is hand-written rather than generated so it can carry ownRecord, which
// gqlgen would otherwise drop. The email resolver uses it to tell a caller's
// own record (signup, verification) apart from a lookup of someone else.
type User struct {
	ID           ID            `json:"id"`
	Name         string        `json:"name"`
	Alias        string        `json:"alias"`
	Email        string        `json:"email"`
	Host         *string       `json:"host,omitempty"`
	Workspace    ID            `json:"workspace"`
	Auths        []string      `json:"auths"`
	Metadata     *UserMetadata `json:"metadata"`
	Verification *Verification `json:"verification,omitempty"`

	ownRecord bool
}

func (User) IsNode()     {}
func (u User) GetID() ID { return u.ID }

func (u *User) OwnRecord() bool { return u.ownRecord }
