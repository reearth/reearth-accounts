package session

import (
	"testing"
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
	"github.com/stretchr/testify/assert"
)

func TestManager_IssueParse_RoundTrip(t *testing.T) {
	m := NewManager("test-secret-that-is-long-enough-000", time.Hour)
	id := adminuser.NewID()
	now := time.Now()

	tok, err := m.Issue(id, now)
	assert.NoError(t, err)
	assert.NotEmpty(t, tok)

	got, err := m.Parse(tok)
	assert.NoError(t, err)
	assert.Equal(t, id, got)
}

func TestManager_Parse_Expired(t *testing.T) {
	m := NewManager("test-secret-that-is-long-enough-000", time.Hour)
	id := adminuser.NewID()

	tok, err := m.Issue(id, time.Now().Add(-2*time.Hour)) // expired 1h ago
	assert.NoError(t, err)

	_, err = m.Parse(tok)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestManager_Parse_WrongSecret(t *testing.T) {
	m := NewManager("secret-a-secret-a-secret-a-secret-a", time.Hour)
	other := NewManager("secret-b-secret-b-secret-b-secret-b", time.Hour)

	tok, err := m.Issue(adminuser.NewID(), time.Now())
	assert.NoError(t, err)

	_, err = other.Parse(tok)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestManager_Parse_Garbage(t *testing.T) {
	m := NewManager("test-secret-that-is-long-enough-000", time.Hour)
	_, err := m.Parse("not-a-jwt")
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestManager_EmptySecret(t *testing.T) {
	m := NewManager("", time.Hour)
	_, err := m.Issue(adminuser.NewID(), time.Now())
	assert.ErrorIs(t, err, ErrEmptySecret)
}
