package workspace

import (
	"testing"
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/sso"
	"github.com/stretchr/testify/assert"
)

func TestWorkspace_ID(t *testing.T) {
	wid := NewID()
	assert.Equal(t, wid, (&Workspace{id: wid}).ID())
}

func TestWorkspace_Name(t *testing.T) {
	assert.Equal(t, "x", (&Workspace{name: "x"}).Name())
}
func TestWorkspace_Alias(t *testing.T) {
	assert.Equal(t, "x", (&Workspace{alias: "x"}).Alias())
}
func TestWorkspace_Members(t *testing.T) {
	m := NewMembersWith(map[UserID]Member{
		NewUserID(): {Role: role.RoleOwner},
	}, nil, false)
	assert.Equal(t, m, (&Workspace{members: m}).Members())
}

func TestWorkspace_IsPersonal(t *testing.T) {
	m := NewMembersWith(map[UserID]Member{
		NewUserID(): {Role: role.RoleOwner},
	}, nil, true)
	assert.True(t, (&Workspace{members: m}).IsPersonal())
	assert.False(t, (&Workspace{}).IsPersonal())
}

func TestWorkspace_Rename(t *testing.T) {
	w := &Workspace{}
	w.Rename("a")
	assert.Equal(t, "a", w.name)
}

func TestWorkspace_UpdateAlias(t *testing.T) {
	w := &Workspace{}
	w.UpdateAlias("a")
	assert.Equal(t, "a", w.alias)
}

func TestWorkspace_UpdateEmail(t *testing.T) {
	w := &Workspace{}
	w.UpdateEmail("a")
	assert.Equal(t, "a", w.email)
}

func TestWorkspace_Policy(t *testing.T) {
	w := &Workspace{}
	w.SetPolicy(PolicyID("ccc").Ref())
	assert.Equal(t, PolicyID("ccc").Ref(), w.Policy())
}

func TestWorkspace_UpdatedAt(t *testing.T) {
	w := &Workspace{}
	now := time.Now()

	// Test Rename updates timestamp
	w.Rename("newname")
	assert.False(t, w.updatedAt.IsZero())
	assert.True(t, w.updatedAt.After(now) || w.updatedAt.Equal(now))

	// Test UpdateAlias updates timestamp
	prevTime := w.updatedAt
	time.Sleep(time.Millisecond)
	w.UpdateAlias("newalias")
	assert.True(t, w.updatedAt.After(prevTime))

	// Test UpdateEmail updates timestamp
	prevTime = w.updatedAt
	time.Sleep(time.Millisecond)
	w.UpdateEmail("new@example.com")
	assert.True(t, w.updatedAt.After(prevTime))

	// Test SetMetadata updates timestamp
	prevTime = w.updatedAt
	time.Sleep(time.Millisecond)
	w.SetMetadata(NewMetadata())
	assert.True(t, w.updatedAt.After(prevTime))

	// Test SetPolicy updates timestamp
	prevTime = w.updatedAt
	time.Sleep(time.Millisecond)
	w.SetPolicy(PolicyID("test").Ref())
	assert.True(t, w.updatedAt.After(prevTime))
}

func TestWorkspace_UpdatedAt_Getter(t *testing.T) {
	now := time.Now()
	w := &Workspace{
		id:        NewID(),
		name:      "test",
		updatedAt: now,
	}

	assert.Equal(t, now, w.UpdatedAt())
}

func TestWorkspace_IsEnterprise(t *testing.T) {
	p := PolicyEnterprise

	enterprise := New().NewID().Name("Enterprise Corp").Personal(false).Policy(&p).MustBuild()
	assert.True(t, enterprise.IsEnterprise())

	free := New().NewID().Name("Free Workspace").Personal(false).MustBuild()
	assert.False(t, free.IsEnterprise())

	other := PolicyID("starter")
	other2 := New().NewID().Name("Starter").Personal(false).Policy(&other).MustBuild()
	assert.False(t, other2.IsEnterprise())
}

func TestWorkspace_SSOConfig(t *testing.T) {
	ws := New().NewID().Name("Test").Personal(false).MustBuild()
	assert.Nil(t, ws.SSOConfig())

	cfg := sso.New(sso.ConnectionTypeSAML)
	cfg.SetEnabled(true)
	cfg.SetEmailDomains([]string{"corp.com"})

	ws.SetSSOConfig(cfg)
	assert.NotNil(t, ws.SSOConfig())
	assert.True(t, ws.SSOConfig().Enabled())

	ws.DeleteSSOConfig()
	assert.Nil(t, ws.SSOConfig())
}
