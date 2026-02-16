package gqlmodel_test

import (
	"context"
	"testing"

	"github.com/hasura/go-graphql-client"
	"github.com/reearth/reearth-accounts/server/pkg/gqlclient/gqlmodel"
	"github.com/stretchr/testify/assert"
)

func TestToUser(t *testing.T) {
	ctx := context.Background()

	t.Run("successfully convert user", func(t *testing.T) {
		gqlUser := gqlmodel.User{
			ID:        "01j9x0yy00000000000000000a",
			Alias:     "testuser",
			Name:      "Test User",
			Email:     "test@example.com",
			Workspace: "01j9x0yy00000000000000001a",
			Auths:     []graphql.String{"auth0|123456"},
			Metadata: gqlmodel.UserMetadata{
				PhotoURL:    "https://example.com/photo.jpg",
				Description: "Test description",
				Website:     "https://example.com",
				Lang:        "en",
				Theme:       "light",
			},
		}

		user, err := gqlmodel.ToUser(ctx, gqlUser)

		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, "Test User", user.Name())
		assert.Equal(t, "testuser", user.Alias())
		assert.Equal(t, "test@example.com", user.Email())
		assert.Equal(t, "https://example.com/photo.jpg", user.Metadata().PhotoURL())
		assert.Equal(t, "Test description", user.Metadata().Description())
		assert.Equal(t, "https://example.com", user.Metadata().Website())
		assert.Equal(t, "en", user.Metadata().Lang().String())
		assert.Equal(t, "light", string(user.Metadata().Theme()))
	})

	t.Run("successfully convert user with empty metadata", func(t *testing.T) {
		gqlUser := gqlmodel.User{
			ID:        "01j9x0yy00000000000000000a",
			Alias:     "testuser",
			Name:      "Test User",
			Email:     "test@example.com",
			Workspace: "01j9x0yy00000000000000001a",
			Auths:     []graphql.String{},
			Metadata: gqlmodel.UserMetadata{
				PhotoURL:    "",
				Description: "",
				Website:     "",
				Lang:        "",
				Theme:       "",
			},
		}

		user, err := gqlmodel.ToUser(ctx, gqlUser)

		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, "Test User", user.Name())
		assert.Equal(t, "", user.Metadata().PhotoURL())
		assert.Equal(t, "", user.Metadata().Description())
		assert.Equal(t, "", user.Metadata().Website())
	})

	t.Run("error on invalid user ID", func(t *testing.T) {
		gqlUser := gqlmodel.User{
			ID:        "invalid-id",
			Alias:     "testuser",
			Name:      "Test User",
			Email:     "test@example.com",
			Workspace: "01j9x0yy00000000000000001a",
			Auths:     []graphql.String{},
			Metadata:  gqlmodel.UserMetadata{},
		}

		user, err := gqlmodel.ToUser(ctx, gqlUser)

		assert.Error(t, err)
		assert.Nil(t, user)
	})

	t.Run("error on invalid workspace ID", func(t *testing.T) {
		gqlUser := gqlmodel.User{
			ID:        "01j9x0yy00000000000000000a",
			Alias:     "testuser",
			Name:      "Test User",
			Email:     "test@example.com",
			Workspace: "invalid-workspace-id",
			Auths:     []graphql.String{},
			Metadata:  gqlmodel.UserMetadata{},
		}

		user, err := gqlmodel.ToUser(ctx, gqlUser)

		assert.Error(t, err)
		assert.Nil(t, user)
	})

	t.Run("successfully convert user with japanese metadata", func(t *testing.T) {
		gqlUser := gqlmodel.User{
			ID:        "01j9x0yy00000000000000000a",
			Alias:     "testuser",
			Name:      "テストユーザー",
			Email:     "test@example.com",
			Workspace: "01j9x0yy00000000000000001a",
			Auths:     []graphql.String{"auth0|123456"},
			Metadata: gqlmodel.UserMetadata{
				PhotoURL:    "https://example.com/photo.jpg",
				Description: "テスト説明",
				Website:     "https://example.jp",
				Lang:        "ja",
				Theme:       "dark",
			},
		}

		user, err := gqlmodel.ToUser(ctx, gqlUser)

		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, "テストユーザー", user.Name())
		assert.Equal(t, "ja", user.Metadata().Lang().String())
		assert.Equal(t, "dark", string(user.Metadata().Theme()))
	})
}

func TestToUsers(t *testing.T) {
	ctx := context.Background()

	t.Run("successfully convert multiple users", func(t *testing.T) {
		gqlUsers := []gqlmodel.User{
			{
				ID:        "01j9x0yy00000000000000000a",
				Alias:     "user1",
				Name:      "User One",
				Email:     "user1@example.com",
				Workspace: "01j9x0yy00000000000000001a",
				Auths:     []graphql.String{"auth0|111111"},
				Metadata: gqlmodel.UserMetadata{
					PhotoURL:    "https://example.com/photo1.jpg",
					Description: "User one description",
					Website:     "https://example1.com",
					Lang:        "en",
					Theme:       "light",
				},
			},
			{
				ID:        "01j9x0yy00000000000000000b",
				Alias:     "user2",
				Name:      "User Two",
				Email:     "user2@example.com",
				Workspace: "01j9x0yy00000000000000001b",
				Auths:     []graphql.String{"auth0|222222"},
				Metadata: gqlmodel.UserMetadata{
					PhotoURL:    "https://example.com/photo2.jpg",
					Description: "User two description",
					Website:     "https://example2.com",
					Lang:        "ja",
					Theme:       "dark",
				},
			},
		}

		users := gqlmodel.ToUsers(ctx, gqlUsers)

		assert.NotNil(t, users)
		assert.Len(t, users, 2)
		assert.Equal(t, "User One", users[0].Name())
		assert.Equal(t, "user1@example.com", users[0].Email())
		assert.Equal(t, "User Two", users[1].Name())
		assert.Equal(t, "user2@example.com", users[1].Email())
	})

	t.Run("successfully convert empty user list", func(t *testing.T) {
		gqlUsers := []gqlmodel.User{}

		users := gqlmodel.ToUsers(ctx, gqlUsers)

		assert.NotNil(t, users)
		assert.Len(t, users, 0)
	})

	t.Run("skip invalid users", func(t *testing.T) {
		gqlUsers := []gqlmodel.User{
			{
				ID:        "01j9x0yy00000000000000000a",
				Alias:     "validuser",
				Name:      "Valid User",
				Email:     "valid@example.com",
				Workspace: "01j9x0yy00000000000000001a",
				Auths:     []graphql.String{},
				Metadata:  gqlmodel.UserMetadata{},
			},
			{
				ID:        "invalid-id",
				Alias:     "invaliduser",
				Name:      "Invalid User",
				Email:     "invalid@example.com",
				Workspace: "01j9x0yy00000000000000001b",
				Auths:     []graphql.String{},
				Metadata:  gqlmodel.UserMetadata{},
			},
		}

		users := gqlmodel.ToUsers(ctx, gqlUsers)

		assert.NotNil(t, users)
		assert.Len(t, users, 1)
		assert.Equal(t, "Valid User", users[0].Name())
	})

	t.Run("skip all invalid users", func(t *testing.T) {
		gqlUsers := []gqlmodel.User{
			{
				ID:        "invalid-id-1",
				Alias:     "user1",
				Name:      "User One",
				Email:     "user1@example.com",
				Workspace: "01j9x0yy00000000000000001a",
				Auths:     []graphql.String{},
				Metadata:  gqlmodel.UserMetadata{},
			},
			{
				ID:        "invalid-id-2",
				Alias:     "user2",
				Name:      "User Two",
				Email:     "user2@example.com",
				Workspace: "01j9x0yy00000000000000001b",
				Auths:     []graphql.String{},
				Metadata:  gqlmodel.UserMetadata{},
			},
		}

		users := gqlmodel.ToUsers(ctx, gqlUsers)

		assert.NotNil(t, users)
		assert.Len(t, users, 0)
	})

	t.Run("skip users with invalid workspace ID", func(t *testing.T) {
		gqlUsers := []gqlmodel.User{
			{
				ID:        "01j9x0yy00000000000000000a",
				Alias:     "validuser",
				Name:      "Valid User",
				Email:     "valid@example.com",
				Workspace: "01j9x0yy00000000000000001a",
				Auths:     []graphql.String{},
				Metadata:  gqlmodel.UserMetadata{},
			},
			{
				ID:        "01j9x0yy00000000000000000b",
				Alias:     "invalidworkspace",
				Name:      "Invalid Workspace User",
				Email:     "invalid@example.com",
				Workspace: "invalid-workspace-id",
				Auths:     []graphql.String{},
				Metadata:  gqlmodel.UserMetadata{},
			},
		}

		users := gqlmodel.ToUsers(ctx, gqlUsers)

		assert.NotNil(t, users)
		assert.Len(t, users, 1)
		assert.Equal(t, "Valid User", users[0].Name())
	})
}
