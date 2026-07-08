package adminuser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func newTestAdminUser() *AdminUser {
	return New().NewID().Name("Alice").Email("alice@eukarya.io").MustBuild()
}

func TestAdminUser_Getters(t *testing.T) {
	u := newTestAdminUser()
	assert.Equal(t, "Alice", u.Name())
	assert.Equal(t, "alice@eukarya.io", u.Email())
	assert.Equal(t, StatusPending, u.Status())
	assert.False(t, u.ID().IsEmpty())
	assert.False(t, u.UpdatedAt().IsZero())
	assert.Equal(t, u.ID().Timestamp(), u.CreatedAt())
	assert.True(t, u.ApprovedAt().IsZero())
	assert.True(t, u.ApprovedBy().IsEmpty())
}

func TestAdminUser_NilSafe(t *testing.T) {
	var u *AdminUser
	assert.Equal(t, ID{}, u.ID())
	assert.Equal(t, "", u.Name())
	assert.Equal(t, "", u.Email())
	assert.Equal(t, "", u.PictureURL())
	assert.Equal(t, Status(""), u.Status())
	assert.True(t, u.CreatedAt().IsZero())
	assert.False(t, u.IsApproved())
	assert.False(t, u.IsPending())
	assert.False(t, u.IsRejected())
}

func TestAdminUser_StatusHelpers(t *testing.T) {
	u := newTestAdminUser()
	assert.True(t, u.IsPending())
	assert.False(t, u.IsApproved())
	assert.False(t, u.IsRejected())
}

func TestAdminUser_Approve(t *testing.T) {
	u := newTestAdminUser()
	before := u.UpdatedAt()
	approver := NewID()

	u.Approve(approver)

	assert.True(t, u.IsApproved())
	assert.Equal(t, approver, u.ApprovedBy())
	assert.False(t, u.ApprovedAt().IsZero())
	assert.False(t, u.UpdatedAt().Before(before))
}

func TestAdminUser_SetRole(t *testing.T) {
	u := newTestAdminUser()
	before := u.UpdatedAt()

	assert.NoError(t, u.SetRole(RoleSystemAdmin))

	assert.Equal(t, RoleSystemAdmin, u.Role())
	assert.False(t, u.UpdatedAt().Before(before))

	// an invalid role is rejected and leaves the user unchanged
	assert.ErrorIs(t, u.SetRole(Role("bogus")), ErrInvalidRole)
	assert.Equal(t, RoleSystemAdmin, u.Role())
}

func TestAdminUser_Approve_Idempotent(t *testing.T) {
	u := newTestAdminUser()
	first := NewID()
	u.Approve(first)
	approvedAt := u.ApprovedAt()

	// re-approving an already-approved user must not overwrite audit data
	u.Approve(NewID())
	assert.Equal(t, first, u.ApprovedBy())
	assert.Equal(t, approvedAt, u.ApprovedAt())
}

func TestAdminUser_Approve_AfterReject(t *testing.T) {
	u := newTestAdminUser()
	u.Approve(NewID())
	u.Reject()
	assert.True(t, u.IsRejected())

	second := NewID()
	u.Approve(second)
	assert.True(t, u.IsApproved())
	assert.Equal(t, second, u.ApprovedBy())
}

func TestAdminUser_Reject(t *testing.T) {
	u := newTestAdminUser()
	approver := NewID()
	u.Approve(approver)

	u.Reject()

	assert.True(t, u.IsRejected())
	// approval history is retained
	assert.Equal(t, approver, u.ApprovedBy())
	assert.False(t, u.ApprovedAt().IsZero())
}

func TestAdminUser_UpdateProfile(t *testing.T) {
	u := newTestAdminUser()

	err := u.UpdateProfile("Alice Example", "https://example.com/a.png")
	assert.NoError(t, err)
	assert.Equal(t, "Alice Example", u.Name())
	assert.Equal(t, "https://example.com/a.png", u.PictureURL())

	err = u.UpdateProfile("", "https://example.com/a.png")
	assert.Equal(t, ErrEmptyName, err)
}

func TestNormalizeEmail(t *testing.T) {
	assert.Equal(t, "alice@eukarya.io", NormalizeEmail("  Alice@Eukarya.io "))
	// display-name form is reduced to the bare address
	assert.Equal(t, "alice@eukarya.io", NormalizeEmail("Alice <Alice@Eukarya.io>"))
	// invalid input is returned lowercased/trimmed unchanged
	assert.Equal(t, "not-an-email", NormalizeEmail("  Not-An-Email "))
	assert.Equal(t, "", NormalizeEmail(""))
}
