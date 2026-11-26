package workspace

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOperatorClientMarker(t *testing.T) {
	uid, iid := NewUserID(), NewIntegrationID()
	c := &Client{ID: "c1", Name: "app"}

	self := &Operator{User: &uid}
	assert.True(t, self.IsUserSelf())
	assert.False(t, self.ViaClient())
	assert.Equal(t, UserSubject(uid), self.Subject())

	integration := &Operator{Integration: &iid, Client: c}
	assert.True(t, integration.IsService())
	assert.True(t, integration.ViaClient())
	assert.False(t, integration.IsUserSelf())
	assert.Equal(t, "integration:"+iid.String(), integration.Subject().String())

	var none *Operator
	assert.False(t, none.ViaClient())
	assert.False(t, none.IsUserSelf())
	assert.Equal(t, SubjectNone, none.Subject().Kind())
	assert.Equal(t, "none", none.Subject().String())
}
