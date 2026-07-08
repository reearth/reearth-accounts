package adminuser

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBuilder_Build(t *testing.T) {
	id := NewID()
	now := time.Now()

	tests := []struct {
		Name     string
		Build    func() (*AdminUser, error)
		Expected *AdminUser
		Err      error
	}{
		{
			Name: "success with defaults",
			Build: func() (*AdminUser, error) {
				return New().ID(id).Name("Alice").Email("Alice@Eukarya.io").Build()
			},
			Expected: &AdminUser{
				id:     id,
				name:   "Alice",
				email:  "alice@eukarya.io",
				status: StatusPending,
			},
		},
		{
			Name: "success approved",
			Build: func() (*AdminUser, error) {
				return New().ID(id).Name("Alice").Email("alice@eukarya.io").
					Status(StatusApproved).UpdatedAt(now).Build()
			},
			Expected: &AdminUser{
				id:        id,
				name:      "Alice",
				email:     "alice@eukarya.io",
				status:    StatusApproved,
				updatedAt: now,
			},
		},
		{
			Name:  "fail nil id",
			Build: func() (*AdminUser, error) { return New().Name("Alice").Email("alice@eukarya.io").Build() },
			Err:   ErrInvalidID,
		},
		{
			Name:  "fail empty name",
			Build: func() (*AdminUser, error) { return New().ID(id).Email("alice@eukarya.io").Build() },
			Err:   ErrEmptyName,
		},
		{
			Name:  "fail invalid email",
			Build: func() (*AdminUser, error) { return New().ID(id).Name("Alice").Email("not-an-email").Build() },
			Err:   ErrInvalidEmail,
		},
		{
			Name: "fail invalid status",
			Build: func() (*AdminUser, error) {
				return New().ID(id).Name("Alice").Email("alice@eukarya.io").Status(Status("bogus")).Build()
			},
			Err: ErrInvalidStatus,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()
			res, err := tt.Build()
			if tt.Err != nil {
				assert.Nil(t, res)
				assert.Equal(t, tt.Err, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.Expected.id, res.id)
			assert.Equal(t, tt.Expected.name, res.name)
			assert.Equal(t, tt.Expected.email, res.email)
			assert.Equal(t, tt.Expected.status, res.status)
			assert.False(t, res.updatedAt.IsZero())
		})
	}
}

func TestBuilder_ApprovedStatusDefaultsApprovedAt(t *testing.T) {
	u := New().NewID().Name("Alice").Email("alice@eukarya.io").Status(StatusApproved).MustBuild()
	assert.True(t, u.IsApproved())
	assert.False(t, u.ApprovedAt().IsZero())
}

func TestBuilder_PendingClearsApprovalMetadata(t *testing.T) {
	u := New().NewID().Name("Alice").Email("alice@eukarya.io").
		Status(StatusPending).
		ApprovedBy(NewID()).
		ApprovedAt(time.Now()).
		MustBuild()
	assert.True(t, u.IsPending())
	assert.True(t, u.ApprovedBy().IsEmpty())
	assert.True(t, u.ApprovedAt().IsZero())
}

func TestBuilder_NormalizesDisplayNameEmail(t *testing.T) {
	u := New().NewID().Name("Alice").Email("Alice <Alice@Eukarya.io>").MustBuild()
	assert.Equal(t, "alice@eukarya.io", u.Email())
}

func TestBuilder_NewID(t *testing.T) {
	u := New().NewID().Name("Bob").Email("bob@eukarya.io").MustBuild()
	assert.False(t, u.ID().IsEmpty())
}

func TestBuilder_Role(t *testing.T) {
	u := New().NewID().Name("Bob").Email("bob@eukarya.io").Role(RoleSystemAdmin).MustBuild()
	assert.Equal(t, RoleSystemAdmin, u.Role())

	// empty means "unset" (pre-migration records) and still builds
	u = New().NewID().Name("Bob").Email("bob@eukarya.io").MustBuild()
	assert.Equal(t, Role(""), u.Role())

	// an invalid role is rejected at construction time
	_, err := New().NewID().Name("Bob").Email("bob@eukarya.io").Role(Role("bogus")).Build()
	assert.ErrorIs(t, err, ErrInvalidRole)
}

func TestBuilder_MustBuild_Panics(t *testing.T) {
	assert.Panics(t, func() {
		New().Name("Bob").Email("bob@eukarya.io").MustBuild()
	})
}
