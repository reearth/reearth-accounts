package memory

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/pkg/id"
	"github.com/reearth/reearth-accounts/pkg/user"
	"github.com/reearth/reearthx/rerror"
	"github.com/stretchr/testify/assert"
)

func TestNewUser(t *testing.T) {
	usr := NewUser()
	assert.NotNil(t, usr)
}

func TestUser_FindByID(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {

		uid := id.NewUserID()
		usr, err := user.New().ID(uid).
			Name("test").
			Email("test@mail.com").
			Build()
		assert.NoError(t, err)

		repo := NewUserWith(usr)
		got, err := repo.FindByID(ctx, uid)
		assert.NoError(t, err)
		assert.Equal(t, usr, got)
	})

	t.Run("not found", func(t *testing.T) {
		repo := NewUser()
		_, err := repo.FindByID(ctx, id.NewUserID())
		assert.Error(t, err)
		assert.Equal(t, rerror.ErrNotFound, err)
	})
}

func TestUser_Save(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		uid := id.NewUserID()
		usr, err := user.New().ID(uid).
			Name("test").
			Email("test@mail.com").
			Build()
		assert.NoError(t, err)
		repo := NewUser()
		err = repo.Save(ctx, usr)
		assert.NoError(t, err)
		got, err := repo.FindByID(ctx, uid)
		assert.NoError(t, err)
		assert.Equal(t, usr, got)
	})
}
