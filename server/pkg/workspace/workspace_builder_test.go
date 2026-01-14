package workspace

import (
	"testing"

	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearthx/idx"
	"github.com/stretchr/testify/assert"
)

func TestBuilder_ID(t *testing.T) {
	wid := NewID()
	b := New().ID(wid)
	assert.Equal(t, wid, b.w.id)
}

func TestBuilder_NewID(t *testing.T) {
	b := New().NewID()
	assert.False(t, b.w.id.IsEmpty())
}

func TestBuilder_ParseID(t *testing.T) {
	id := NewID()
	b := New().ParseID(id.String()).MustBuild()
	assert.Equal(t, id, b.ID())

	_, err := New().ParseID("invalid").Build()
	assert.Equal(t, idx.ErrInvalidID, err)
}

func TestBuilder_Members(t *testing.T) {
	m := map[UserID]Member{NewUserID(): {Role: role.RoleOwner}}
	b := New().Members(m)
	assert.Equal(t, m, b.members)
}

func TestBuilder_Name(t *testing.T) {
	w := New().Name("xxx")
	assert.Equal(t, "xxx", w.w.name)
}

func TestBuilder_Alias(t *testing.T) {
	w := New().Alias("xxx")
	assert.Equal(t, "xxx", w.w.alias)
}

func TestBuilder_Build(t *testing.T) {
	m := map[UserID]Member{NewUserID(): {Role: role.RoleOwner}}
	i := map[IntegrationID]Member{NewIntegrationID(): {Role: role.RoleOwner}}
	id := NewID()
	metadata := NewMetadata()
	metadata.SetDescription("description")
	metadata.SetWebsite("https://example.com")
	metadata.SetLocation("location")
	metadata.SetBillingEmail("billing@mail.com")
	metadata.SetPhotoURL("https://example.com/photo.jpg")

	w, err := New().ID(id).Name("a").Integrations(i).Metadata(metadata).Members(m).Build()
	assert.NoError(t, err)

	// Check fields individually (excluding updatedAt)
	assert.Equal(t, id, w.id)
	assert.Equal(t, "a", w.name)
	assert.Equal(t, metadata, w.metadata)
	assert.NotNil(t, w.members)
	// Check that updatedAt was set
	assert.False(t, w.updatedAt.IsZero())

	w, err = New().ID(id).Name("a").Metadata(metadata).Build()
	assert.NoError(t, err)

	// Check fields individually (excluding updatedAt)
	assert.Equal(t, id, w.id)
	assert.Equal(t, "a", w.name)
	assert.Equal(t, metadata, w.metadata)
	assert.NotNil(t, w.members)
	assert.Empty(t, w.members.users)
	assert.Empty(t, w.members.integrations)
	// Check that updatedAt was set
	assert.False(t, w.updatedAt.IsZero())

	w, err = New().Build()
	assert.Equal(t, ErrInvalidID, err)
	assert.Nil(t, w)
}

// TODO: reapply when migrations 260114000000 and 260114000001 can run
// func TestBuilder_UpdatedAt(t *testing.T) {
// 	now := time.Now()
// 	customTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
//
// 	t.Run("Builder sets default updatedAt when not specified", func(t *testing.T) {
// 		w := New().NewID().Name("test").MustBuild()
// 		assert.False(t, w.updatedAt.IsZero())
// 		assert.True(t, w.updatedAt.After(now) || w.updatedAt.Equal(now))
// 	})
//
// 	t.Run("Builder respects custom updatedAt", func(t *testing.T) {
// 		w := New().NewID().Name("test").UpdatedAt(customTime).MustBuild()
// 		assert.Equal(t, customTime, w.updatedAt)
// 	})
//
// 	t.Run("UpdatedAt getter returns correct value", func(t *testing.T) {
// 		w := New().NewID().Name("test").UpdatedAt(customTime).MustBuild()
// 		assert.Equal(t, customTime, w.UpdatedAt())
// 	})
// }

func TestBuilder_MustBuild(t *testing.T) {
	m := map[UserID]Member{NewUserID(): {Role: role.RoleOwner}}
	i := map[IntegrationID]Member{NewIntegrationID(): {Role: role.RoleOwner}}
	id := NewID()

	metadata := NewMetadata()
	metadata.SetDescription("description")
	metadata.SetWebsite("https://example.com")
	metadata.SetLocation("location")
	metadata.SetBillingEmail("billing@mail.com")
	metadata.SetPhotoURL("https://example.com/photo.jpg")

	w := New().ID(id).Name("a").Integrations(i).Metadata(metadata).Members(m).MustBuild()

	// Check fields individually (excluding updatedAt)
	assert.Equal(t, id, w.id)
	assert.Equal(t, "a", w.name)
	assert.Equal(t, metadata, w.metadata)
	assert.NotNil(t, w.members)
	// Check that updatedAt was set
	assert.False(t, w.updatedAt.IsZero())

	assert.Panics(t, func() { New().MustBuild() })
}

func TestBuilder_Integrations(t *testing.T) {
	i := map[IntegrationID]Member{NewIntegrationID(): {Role: role.RoleOwner}}
	assert.Equal(t, &Builder{
		w:            &Workspace{},
		integrations: i,
	}, New().Integrations(i))
}

func TestBuilder_Personal(t *testing.T) {
	assert.Equal(t, &Builder{
		w:        &Workspace{},
		personal: true,
	}, New().Personal(true))
}

func TestBuilder_Policy(t *testing.T) {
	pid := PolicyID("id")
	assert.Equal(t, &Builder{
		w: &Workspace{
			policy: &pid,
		},
	}, New().Policy(&pid))
}

func TestBuilder_Email(t *testing.T) {
	assert.Equal(t, &Builder{
		w: &Workspace{
			email: "test@mail.com",
		},
	}, New().Email("test@mail.com"))
}

func TestBuilder_Metadata(t *testing.T) {
	md := MetadataFrom("description", "https://example.com", "location", "billing@mail.com", "https://example.com/photo.jpg")
	assert.Equal(t, &Builder{
		w: &Workspace{
			metadata: md,
		},
	}, New().Metadata(md))
}
