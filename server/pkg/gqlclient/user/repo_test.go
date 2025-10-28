package user_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/reearth/reearth-accounts/server/pkg/gqlclient"
	"github.com/stretchr/testify/assert"
)

func TestUserRepo_FindMe(t *testing.T) {
	ctx := context.Background()

	t.Run("successfully find me", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)
		httpmock.RegisterResponder("POST", "https://accounts.example.com/api/graphql",
			func(req *http.Request) (*http.Response, error) {
				bodyBytes, _ := io.ReadAll(req.Body)
				fmt.Printf("\n--- GraphQL Request ---\n%s\n", string(bodyBytes))
				req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				return httpmock.NewStringResponse(http.StatusOK, `{
					"data": {
						"me": {
							"id": "01j9x0yy00000000000000000a",
							"name": "Test User",
							"alias": "testuser",
							"email": "test@example.com",
							"myWorkspaceId": "01j9x0yy00000000000000001a",
							"host": "",
							"auths": ["auth0|123456"],
							"metadata": {
								"photoURL": "https://example.com/photo.jpg",
								"description": "Test description",
								"website": "https://example.com",
								"lang": "en",
								"theme": "light"
							}
						}
					}
				}`), nil
			},
		)

		got, err := client.UserRepo.FindMe(ctx)

		assert.NoError(t, err)
		assert.NotNil(t, got)
		assert.NotEmpty(t, got.ID())
		assert.Equal(t, "Test User", got.Name())
		assert.Equal(t, "testuser", got.Alias())
		assert.Equal(t, "test@example.com", got.Email())
		assert.Equal(t, "https://example.com/photo.jpg", got.Metadata().PhotoURL())
		assert.NotEmpty(t, got.Workspace())
		assert.Len(t, got.Auths(), 1)
	})

	t.Run("invalid user ID", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		httpmock.RegisterResponder(
			"POST",
			"https://accounts.example.com/api/graphql",
			httpmock.NewStringResponder(http.StatusOK, `{
				"data": {
					"me": {
						"id": "invalid-id",
						"name": "Test User",
						"alias": "testuser",
						"email": "test@example.com",
						"myWorkspaceId": "01j9x0yy00000000000000001a",
						"host": "",
						"auths": [],
						"metadata": {
							"photoURL": "",
							"description": "",
							"website": "",
							"lang": "",
							"theme": ""
						}
					}
				}
			}`),
		)

		got, err := client.UserRepo.FindMe(ctx)

		assert.Error(t, err)
		assert.Nil(t, got)
	})

	t.Run("invalid workspace ID", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		httpmock.RegisterResponder(
			"POST",
			"https://accounts.example.com/api/graphql",
			httpmock.NewStringResponder(http.StatusOK, `{
				"data": {
					"me": {
						"id": "01j9x0yy00000000000000000a",
						"name": "Test User",
						"alias": "testuser",
						"email": "test@example.com",
						"myWorkspaceId": "invalid-workspace-id",
						"host": "",
						"auths": [],
						"metadata": {
							"photoURL": "",
							"description": "",
							"website": "",
							"lang": "",
							"theme": ""
						}
					}
				}
			}`),
		)

		got, err := client.UserRepo.FindMe(ctx)

		assert.Error(t, err)
		assert.Nil(t, got)
	})
}
