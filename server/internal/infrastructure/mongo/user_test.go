package mongo

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/pkg/id"
	"github.com/reearth/reearth-accounts/pkg/user"
	"github.com/reearth/reearth-accounts/pkg/workspace"
	"github.com/reearth/reearthx/account/accountdomain"
	"github.com/reearth/reearthx/mongox"
	"github.com/stretchr/testify/assert"
)

func TestUser_FindByID(t *testing.T) {
	c := Connect(t)(t)
	ctx := context.Background()
	client := mongox.NewClientWithDatabase(c)
	assert.NotNil(t, client)

	t.Run("success", func(t *testing.T) {
		uid := id.NewUserID()

		rUser := NewUser(client)
		rWorkspace := NewWorkspace(client)

		mUser := user.NewMetadata()
		mUser.SetDescription("Test user description")
		mUser.SetWebsite("https://user-website.com")
		mUser.SetPhotoURL("https://user-photo.com/photo.jpg")

		wsMain, err := workspace.New().
			ID(id.NewWorkspaceID()).
			Name("test workspace").
			Alias("test-alias").
			Build()
		assert.NoError(t, err)
		err = rWorkspace.Save(ctx, wsMain)
		assert.NoError(t, err)

		usrMain, err := user.New().
			ID(uid).
			Name("test user").
			Email("test@mail.com").
			Metadata(mUser).
			Workspace(accountdomain.WorkspaceID(wsMain.ID())).
			Build()
		assert.NoError(t, err)
		err = rUser.Save(ctx, usrMain)
		assert.NoError(t, err)

		usr, err := rUser.FindByID(ctx, usrMain.ID())
		assert.NoError(t, err)
		assert.NotNil(t, usr)
		assert.Equal(t, uid, usr.ID())
		assert.Equal(t, usrMain.Name(), usr.Name())
		assert.Equal(t, usrMain.Email(), usr.Email())
		assert.NotNil(t, usr.Metadata())
		assert.Equal(t, usrMain.Metadata().Description(), usr.Metadata().Description())
		assert.Equal(t, usrMain.Metadata().Website(), usr.Metadata().Website())
		assert.Equal(t, usrMain.Workspace(), usr.Workspace())
	})
}
