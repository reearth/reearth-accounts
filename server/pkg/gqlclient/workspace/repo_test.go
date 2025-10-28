package workspace_test

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

func TestWorkspaceRepo_FindByUser(t *testing.T) {
	ctx := context.Background()

	t.Run("successfully find workspaces by user", func(t *testing.T) {
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
						"findByUser": [
							{
								"id": "01j9x0yy00000000000000001a",
								"name": "Personal Workspace",
								"alias": "personal",
								"personal": true,
								"metadata": {
									"description": "My personal workspace",
									"website": "https://example.com",
									"location": "Tokyo",
									"billingEmail": "billing@example.com",
									"photoURL": "https://example.com/photo.jpg"
								},
								"members": [
									{
										"userId": "01j9x0yy00000000000000000a",
										"role": "OWNER"
									}
								]
							},
							{
								"id": "01j9x0yy00000000000000002a",
								"name": "Team Workspace",
								"alias": "team",
								"personal": false,
								"metadata": {
									"description": "Team collaboration workspace",
									"website": "https://team.example.com",
									"location": "Osaka",
									"billingEmail": "team@example.com",
									"photoURL": "https://team.example.com/photo.jpg"
								},
								"members": [
									{
										"userId": "01j9x0yy00000000000000000a",
										"role": "WRITER"
									},
									{
										"userId": "01j9x0yy00000000000000000b",
										"role": "OWNER"
									}
								]
							}
						]
					}
				}`), nil
			},
		)

		got, err := client.WorkspaceRepo.FindByUser(ctx, "01j9x0yy00000000000000000a")

		assert.NoError(t, err)
		assert.NotNil(t, got)
		assert.Len(t, got, 2)

		// Check first workspace
		assert.Equal(t, "Personal Workspace", got[0].Name())
		assert.Equal(t, "personal", got[0].Alias())
		assert.True(t, got[0].IsPersonal())
		assert.Equal(t, "My personal workspace", got[0].Metadata().Description())
		assert.Equal(t, "https://example.com", got[0].Metadata().Website())
		assert.Equal(t, "Tokyo", got[0].Metadata().Location())
		assert.Equal(t, "billing@example.com", got[0].Metadata().BillingEmail())
		assert.Equal(t, "https://example.com/photo.jpg", got[0].Metadata().PhotoURL())

		// Check second workspace
		assert.Equal(t, "Team Workspace", got[1].Name())
		assert.Equal(t, "team", got[1].Alias())
		assert.False(t, got[1].IsPersonal())
		assert.Equal(t, "Team collaboration workspace", got[1].Metadata().Description())
		assert.Equal(t, "https://team.example.com", got[1].Metadata().Website())
		assert.Equal(t, "Osaka", got[1].Metadata().Location())
		assert.Equal(t, "team@example.com", got[1].Metadata().BillingEmail())
		assert.Equal(t, "https://team.example.com/photo.jpg", got[1].Metadata().PhotoURL())
	})

	t.Run("empty workspace list", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)
		httpmock.RegisterResponder("POST", "https://accounts.example.com/api/graphql",
			func(req *http.Request) (*http.Response, error) {
				return httpmock.NewStringResponse(http.StatusOK, `{
					"data": {
						"findByUser": []
					}
				}`), nil
			},
		)

		got, err := client.WorkspaceRepo.FindByUser(ctx, "01j9x0yy00000000000000000b")

		assert.NoError(t, err)
		assert.NotNil(t, got)
		assert.Len(t, got, 0)
	})

	t.Run("graphql error", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)
		httpmock.RegisterResponder("POST", "https://accounts.example.com/api/graphql",
			func(req *http.Request) (*http.Response, error) {
				return httpmock.NewStringResponse(http.StatusOK, `{
					"errors": [
						{
							"message": "User not found",
							"path": ["findByUser"]
						}
					]
				}`), nil
			},
		)

		got, err := client.WorkspaceRepo.FindByUser(ctx, "invalid-user-id")

		assert.Error(t, err)
		assert.Nil(t, got)
	})
}
