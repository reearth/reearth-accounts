package workspace

import (
	"testing"

	accountsid "github.com/reearth/reearth-accounts/pkg/id"
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
	m := map[UserID]Member{NewUserID(): {Role: RoleOwner}}
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
	m := map[UserID]Member{NewUserID(): {Role: RoleOwner}}
	i := map[IntegrationID]Member{NewIntegrationID(): {Role: RoleOwner}}
	id := NewID()
	metadata := NewMetadata()
	metadata.SetDescription("description")
	metadata.SetWebsite("https://example.com")
	metadata.SetLocation("location")
	metadata.SetBillingEmail("billing@mail.com")
	metadata.SetPhotoURL("https://example.com/photo.jpg")

	w, err := New().ID(id).Name("a").Integrations(i).Metadata(metadata).Members(m).Build()
	assert.NoError(t, err)

	assert.Equal(t, &Workspace{
		id:       id,
		name:     "a",
		members:  NewMembersWith(m, i, false),
		metadata: metadata,
	}, w)

	w, err = New().ID(id).Name("a").Metadata(metadata).Build()
	assert.NoError(t, err)

	assert.Equal(t, &Workspace{
		id:   id,
		name: "a",
		members: &Members{
			users:        map[idx.ID[accountsid.User]]Member{},
			integrations: map[idx.ID[accountsid.Integration]]Member{},
		},
		metadata: metadata,
	}, w)

	w, err = New().Build()
	assert.Equal(t, ErrInvalidID, err)
	assert.Nil(t, w)
}

func TestBuilder_MustBuild(t *testing.T) {
	m := map[UserID]Member{NewUserID(): {Role: RoleOwner}}
	i := map[IntegrationID]Member{NewIntegrationID(): {Role: RoleOwner}}
	id := NewID()

	metadata := NewMetadata()
	metadata.SetDescription("description")
	metadata.SetWebsite("https://example.com")
	metadata.SetLocation("location")
	metadata.SetBillingEmail("billing@mail.com")
	metadata.SetPhotoURL("https://example.com/photo.jpg")

	w := New().ID(id).Name("a").Integrations(i).Metadata(metadata).Members(m).MustBuild()

	assert.Equal(t, &Workspace{
		id:       id,
		name:     "a",
		members:  NewMembersWith(m, i, false),
		metadata: metadata,
	}, w)

	assert.Panics(t, func() { New().MustBuild() })
}

func TestBuilder_Integrations(t *testing.T) {
	i := map[IntegrationID]Member{NewIntegrationID(): {Role: RoleOwner}}
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
