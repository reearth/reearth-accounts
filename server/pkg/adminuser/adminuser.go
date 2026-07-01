package adminuser

import (
	"errors"
	"net/mail"
	"strings"
	"time"
)

var (
	ErrEmptyName    = errors.New("admin user name can't be empty")
	ErrInvalidEmail = errors.New("invalid email")
)

// AdminUser is an administrator of the admin application. Admin users sign in
// via Google OAuth (restricted to an allowed domain) and must be approved by an
// existing approved admin before they can access admin features.
type AdminUser struct {
	approvedAt time.Time // zero value if not yet approved
	approvedBy ID        // zero value if not yet approved
	email      string    // unique, lowercase
	id         ID
	name       string
	pictureURL string
	status     Status
	updatedAt  time.Time
}

func (u *AdminUser) ID() ID {
	if u == nil {
		return ID{}
	}
	return u.id
}

func (u *AdminUser) ApprovedAt() time.Time {
	if u == nil {
		return time.Time{}
	}
	return u.approvedAt
}

func (u *AdminUser) ApprovedBy() ID {
	if u == nil {
		return ID{}
	}
	return u.approvedBy
}

// CreatedAt is derived from the ULID-based ID, which embeds its creation time.
func (u *AdminUser) CreatedAt() time.Time {
	if u == nil {
		return time.Time{}
	}
	return u.id.Timestamp()
}

func (u *AdminUser) Email() string {
	if u == nil {
		return ""
	}
	return u.email
}

func (u *AdminUser) Name() string {
	if u == nil {
		return ""
	}
	return u.name
}

func (u *AdminUser) PictureURL() string {
	if u == nil {
		return ""
	}
	return u.pictureURL
}

func (u *AdminUser) Status() Status {
	if u == nil {
		return ""
	}
	return u.status
}

func (u *AdminUser) UpdatedAt() time.Time {
	if u == nil {
		return time.Time{}
	}
	return u.updatedAt
}

func (u *AdminUser) IsApproved() bool {
	return u != nil && u.status == StatusApproved
}

func (u *AdminUser) IsPending() bool {
	return u != nil && u.status == StatusPending
}

func (u *AdminUser) IsRejected() bool {
	return u != nil && u.status == StatusRejected
}

// Approve marks the user as approved and records who approved it and when.
func (u *AdminUser) Approve(by ID) {
	if u == nil {
		return
	}
	now := time.Now()
	u.status = StatusApproved
	u.approvedBy = by
	u.approvedAt = now
	u.updatedAt = now
}

// Reject marks the user as rejected. It is used both to reject a pending user
// and to revoke an already-approved one. Approval history (approvedBy /
// approvedAt) is intentionally retained.
func (u *AdminUser) Reject() {
	if u == nil {
		return
	}
	u.status = StatusRejected
	u.updatedAt = time.Now()
}

// UpdateProfile refreshes the display name and picture from the Google profile
// on subsequent sign-ins.
func (u *AdminUser) UpdateProfile(name, pictureURL string) error {
	if u == nil {
		return nil
	}
	if name == "" {
		return ErrEmptyName
	}
	u.name = name
	u.pictureURL = pictureURL
	u.updatedAt = time.Now()
	return nil
}

// NormalizeEmail parses an email, strips any display-name portion, then
// lowercases and trims the address so it can be used as a stable,
// case-insensitive unique key. Invalid input is returned lowercased/trimmed
// unchanged so callers using it purely as a lookup key still get a stable form.
func NormalizeEmail(email string) string {
	if addr, err := mail.ParseAddress(email); err == nil {
		return strings.ToLower(strings.TrimSpace(addr.Address))
	}
	return strings.ToLower(strings.TrimSpace(email))
}

// normalizeAndValidateEmail validates an email and returns its canonical form
// (address only, lowercased, trimmed).
func normalizeAndValidateEmail(email string) (string, error) {
	addr, err := mail.ParseAddress(email)
	if err != nil {
		return "", ErrInvalidEmail
	}
	return strings.ToLower(strings.TrimSpace(addr.Address)), nil
}
