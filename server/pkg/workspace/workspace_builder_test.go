package workspace

import (
	"testing"

	accountsid "github.com/reearth/reearth-accounts/server/pkg/id"
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

func TestBuilder_DisplayName(t *testing.T) {
	w := New().DisplayName("xxx")
	assert.Equal(t, "xxx", w.w.displayName)
}

func TestBuilder_Build(t *testing.T) {
	m := map[UserID]Member{NewUserID(): {Role: RoleOwner}}
	i := map[IntegrationID]Member{NewIntegrationID(): {Role: RoleOwner}}
	id := NewID()
	w, err := New().ID(id).Name("a").Integrations(i).Members(m).Build()
	assert.NoError(t, err)

	assert.Equal(t, &Workspace{
		id:      id,
		name:    "a",
		members: NewMembersWith(m, i, false),
	}, w)

	w, err = New().ID(id).Name("a").Build()
	assert.NoError(t, err)

	assert.Equal(t, &Workspace{
		id:   id,
		name: "a",
		members: &Members{
			users:        map[idx.ID[accountsid.User]]Member{},
			integrations: map[idx.ID[accountsid.Integration]]Member{},
		},
	}, w)

	w, err = New().Build()
	assert.Equal(t, ErrInvalidID, err)
	assert.Nil(t, w)
}

func TestBuilder_MustBuild(t *testing.T) {
	m := map[UserID]Member{NewUserID(): {Role: RoleOwner}}
	i := map[IntegrationID]Member{NewIntegrationID(): {Role: RoleOwner}}
	id := NewID()
	w := New().ID(id).Name("a").Integrations(i).Members(m).MustBuild()

	assert.Equal(t, &Workspace{
		id:      id,
		name:    "a",
		members: NewMembersWith(m, i, false),
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
