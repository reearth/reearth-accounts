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
	accountsGqlWorkspace "github.com/reearth/reearth-accounts/server/pkg/gqlclient/workspace"
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
										"role": "owner"
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
										"role": "owner"
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

func TestWorkspaceRepo_CreateWorkspace(t *testing.T) {
	ctx := context.Background()

	t.Run("successfully create workspace with all fields", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		description := "Test workspace description"

		httpmock.RegisterResponder("POST", "https://accounts.example.com/api/graphql",
			func(req *http.Request) (*http.Response, error) {
				bodyBytes, _ := io.ReadAll(req.Body)
				fmt.Printf("\n--- CreateWorkspace GraphQL Request ---\n%s\n", string(bodyBytes))
				req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				return httpmock.NewStringResponse(http.StatusOK, `{
					"data": {
						"createWorkspace": {
							"workspace": {
								"id": "01j9x0yy00000000000000001w",
								"name": "New Workspace",
								"alias": "new-workspace",
								"personal": false
							}
						}
					}
				}`), nil
			},
		)

		input := accountsGqlWorkspace.CreateWorkspaceInput{
			Alias:       "new-workspace",
			Name:        "New Workspace",
			Description: &description,
		}

		got, err := client.WorkspaceRepo.CreateWorkspace(ctx, input)

		assert.NoError(t, err)
		assert.NotNil(t, got)
		assert.NotEmpty(t, got.ID())
		assert.Equal(t, "New Workspace", got.Name())
		assert.Equal(t, "new-workspace", got.Alias())
		assert.False(t, got.IsPersonal())
	})

	t.Run("successfully create workspace without description", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		httpmock.RegisterResponder("POST", "https://accounts.example.com/api/graphql",
			func(req *http.Request) (*http.Response, error) {
				bodyBytes, _ := io.ReadAll(req.Body)
				fmt.Printf("\n--- CreateWorkspace No Description GraphQL Request ---\n%s\n", string(bodyBytes))
				req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				return httpmock.NewStringResponse(http.StatusOK, `{
					"data": {
						"createWorkspace": {
							"workspace": {
								"id": "01j9x0yy00000000000000001w",
								"name": "Minimal Workspace",
								"alias": "minimal-workspace",
								"personal": false
							}
						}
					}
				}`), nil
			},
		)

		input := accountsGqlWorkspace.CreateWorkspaceInput{
			Alias:       "minimal-workspace",
			Name:        "Minimal Workspace",
			Description: nil,
		}

		got, err := client.WorkspaceRepo.CreateWorkspace(ctx, input)

		assert.NoError(t, err)
		assert.NotNil(t, got)
		assert.Equal(t, "Minimal Workspace", got.Name())
	})

	t.Run("error on invalid workspace ID in response", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		httpmock.RegisterResponder(
			"POST",
			"https://accounts.example.com/api/graphql",
			httpmock.NewStringResponder(http.StatusOK, `{
				"data": {
					"createWorkspace": {
						"workspace": {
							"id": "invalid-id",
							"name": "Test Workspace",
							"alias": "test-workspace",
							"personal": false
						}
					}
				}
			}`),
		)

		input := accountsGqlWorkspace.CreateWorkspaceInput{
			Alias: "test-workspace",
			Name:  "Test Workspace",
		}

		got, err := client.WorkspaceRepo.CreateWorkspace(ctx, input)

		assert.Error(t, err)
		assert.Nil(t, got)
	})

	t.Run("graphql error", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		httpmock.RegisterResponder(
			"POST",
			"https://accounts.example.com/api/graphql",
			httpmock.NewStringResponder(http.StatusOK, `{
				"errors": [
					{
						"message": "Alias already exists",
						"path": ["createWorkspace"]
					}
				]
			}`),
		)

		input := accountsGqlWorkspace.CreateWorkspaceInput{
			Alias: "existing-alias",
			Name:  "Test Workspace",
		}

		got, err := client.WorkspaceRepo.CreateWorkspace(ctx, input)

		assert.Error(t, err)
		assert.Nil(t, got)
	})
}

func TestWorkspaceRepo_UpdateWorkspace(t *testing.T) {
	ctx := context.Background()

	t.Run("successfully update workspace", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		httpmock.RegisterResponder("POST", "https://accounts.example.com/api/graphql",
			func(req *http.Request) (*http.Response, error) {
				bodyBytes, _ := io.ReadAll(req.Body)
				fmt.Printf("\n--- UpdateWorkspace GraphQL Request ---\n%s\n", string(bodyBytes))
				req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				return httpmock.NewStringResponse(http.StatusOK, `{
					"data": {
						"updateWorkspace": {
							"workspace": {
								"id": "01j9x0yy00000000000000001w",
								"name": "Updated Workspace",
								"alias": "test-workspace",
								"personal": false
							}
						}
					}
				}`), nil
			},
		)

		input := accountsGqlWorkspace.UpdateWorkspaceInput{
			WorkspaceID: "01j9x0yy00000000000000001w",
			Name:        "Updated Workspace",
		}

		got, err := client.WorkspaceRepo.UpdateWorkspace(ctx, input)

		assert.NoError(t, err)
		assert.NotNil(t, got)
		assert.Equal(t, "Updated Workspace", got.Name())
		assert.Equal(t, "test-workspace", got.Alias())
	})

	t.Run("error on invalid workspace ID in response", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		httpmock.RegisterResponder(
			"POST",
			"https://accounts.example.com/api/graphql",
			httpmock.NewStringResponder(http.StatusOK, `{
				"data": {
					"updateWorkspace": {
						"workspace": {
							"id": "invalid-id",
							"name": "Updated Workspace",
							"alias": "test-workspace",
							"personal": false
						}
					}
				}
			}`),
		)

		input := accountsGqlWorkspace.UpdateWorkspaceInput{
			WorkspaceID: "01j9x0yy00000000000000001w",
			Name:        "Updated Workspace",
		}

		got, err := client.WorkspaceRepo.UpdateWorkspace(ctx, input)

		assert.Error(t, err)
		assert.Nil(t, got)
	})

	t.Run("graphql error", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		httpmock.RegisterResponder(
			"POST",
			"https://accounts.example.com/api/graphql",
			httpmock.NewStringResponder(http.StatusOK, `{
				"errors": [
					{
						"message": "Workspace not found",
						"path": ["updateWorkspace"]
					}
				]
			}`),
		)

		input := accountsGqlWorkspace.UpdateWorkspaceInput{
			WorkspaceID: "01j9x0yy00000000000000001w",
			Name:        "Updated Workspace",
		}

		got, err := client.WorkspaceRepo.UpdateWorkspace(ctx, input)

		assert.Error(t, err)
		assert.Nil(t, got)
	})
}

func TestWorkspaceRepo_DeleteWorkspace(t *testing.T) {
	ctx := context.Background()

	t.Run("successfully delete workspace", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		httpmock.RegisterResponder("POST", "https://accounts.example.com/api/graphql",
			func(req *http.Request) (*http.Response, error) {
				bodyBytes, _ := io.ReadAll(req.Body)
				fmt.Printf("\n--- DeleteWorkspace GraphQL Request ---\n%s\n", string(bodyBytes))
				req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				return httpmock.NewStringResponse(http.StatusOK, `{
					"data": {
						"deleteWorkspace": {
							"workspaceId": "01j9x0yy00000000000000001w"
						}
					}
				}`), nil
			},
		)

		err := client.WorkspaceRepo.DeleteWorkspace(ctx, "01j9x0yy00000000000000001w")

		assert.NoError(t, err)
	})

	t.Run("graphql error", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		httpmock.RegisterResponder(
			"POST",
			"https://accounts.example.com/api/graphql",
			httpmock.NewStringResponder(http.StatusOK, `{
				"errors": [
					{
						"message": "Workspace not found",
						"path": ["deleteWorkspace"]
					}
				]
			}`),
		)

		err := client.WorkspaceRepo.DeleteWorkspace(ctx, "01j9x0yy00000000000000001w")

		assert.Error(t, err)
	})
}

func TestWorkspaceRepo_AddUsersToWorkspace(t *testing.T) {
	ctx := context.Background()

	t.Run("successfully add single user", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		httpmock.RegisterResponder("POST", "https://accounts.example.com/api/graphql",
			func(req *http.Request) (*http.Response, error) {
				bodyBytes, _ := io.ReadAll(req.Body)
				fmt.Printf("\n--- AddUsersToWorkspace GraphQL Request ---\n%s\n", string(bodyBytes))
				req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				return httpmock.NewStringResponse(http.StatusOK, `{
					"data": {
						"addUsersToWorkspace": {
							"workspace": {
								"id": "01j9x0yy00000000000000001w",
								"name": "Test Workspace",
								"alias": "test-workspace",
								"personal": false
							}
						}
					}
				}`), nil
			},
		)

		input := accountsGqlWorkspace.AddUsersToWorkspaceInput{
			WorkspaceID: "01j9x0yy00000000000000001w",
			Users: []accountsGqlWorkspace.MemberInput{
				{
					UserID: "01j9x0yy00000000000000000a",
					Role:   "READER",
				},
			},
		}

		got, err := client.WorkspaceRepo.AddUsersToWorkspace(ctx, input)

		assert.NoError(t, err)
		assert.NotNil(t, got)
		assert.Equal(t, "Test Workspace", got.Name())
	})

	t.Run("successfully add multiple users", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		httpmock.RegisterResponder("POST", "https://accounts.example.com/api/graphql",
			func(req *http.Request) (*http.Response, error) {
				bodyBytes, _ := io.ReadAll(req.Body)
				fmt.Printf("\n--- AddUsersToWorkspace Multiple GraphQL Request ---\n%s\n", string(bodyBytes))
				req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				return httpmock.NewStringResponse(http.StatusOK, `{
					"data": {
						"addUsersToWorkspace": {
							"workspace": {
								"id": "01j9x0yy00000000000000001w",
								"name": "Test Workspace",
								"alias": "test-workspace",
								"personal": false
							}
						}
					}
				}`), nil
			},
		)

		input := accountsGqlWorkspace.AddUsersToWorkspaceInput{
			WorkspaceID: "01j9x0yy00000000000000001w",
			Users: []accountsGqlWorkspace.MemberInput{
				{
					UserID: "01j9x0yy00000000000000000a",
					Role:   "READER",
				},
				{
					UserID: "01j9x0yy00000000000000000b",
					Role:   "WRITER",
				},
			},
		}

		got, err := client.WorkspaceRepo.AddUsersToWorkspace(ctx, input)

		assert.NoError(t, err)
		assert.NotNil(t, got)
	})

	t.Run("error on invalid workspace ID in response", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		httpmock.RegisterResponder(
			"POST",
			"https://accounts.example.com/api/graphql",
			httpmock.NewStringResponder(http.StatusOK, `{
				"data": {
					"addUsersToWorkspace": {
						"workspace": {
							"id": "invalid-id",
							"name": "Test Workspace",
							"alias": "test-workspace",
							"personal": false
						}
					}
				}
			}`),
		)

		input := accountsGqlWorkspace.AddUsersToWorkspaceInput{
			WorkspaceID: "01j9x0yy00000000000000001w",
			Users: []accountsGqlWorkspace.MemberInput{
				{
					UserID: "01j9x0yy00000000000000000a",
					Role:   "READER",
				},
			},
		}

		got, err := client.WorkspaceRepo.AddUsersToWorkspace(ctx, input)

		assert.Error(t, err)
		assert.Nil(t, got)
	})

	t.Run("graphql error", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		httpmock.RegisterResponder(
			"POST",
			"https://accounts.example.com/api/graphql",
			httpmock.NewStringResponder(http.StatusOK, `{
				"errors": [
					{
						"message": "User not found",
						"path": ["addUsersToWorkspace"]
					}
				]
			}`),
		)

		input := accountsGqlWorkspace.AddUsersToWorkspaceInput{
			WorkspaceID: "01j9x0yy00000000000000001w",
			Users: []accountsGqlWorkspace.MemberInput{
				{
					UserID: "01j9x0yy00000000000000000a",
					Role:   "READER",
				},
			},
		}

		got, err := client.WorkspaceRepo.AddUsersToWorkspace(ctx, input)

		assert.Error(t, err)
		assert.Nil(t, got)
	})
}

func TestWorkspaceRepo_RemoveUserFromWorkspace(t *testing.T) {
	ctx := context.Background()

	t.Run("successfully remove user", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		httpmock.RegisterResponder("POST", "https://accounts.example.com/api/graphql",
			func(req *http.Request) (*http.Response, error) {
				bodyBytes, _ := io.ReadAll(req.Body)
				fmt.Printf("\n--- RemoveUserFromWorkspace GraphQL Request ---\n%s\n", string(bodyBytes))
				req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				return httpmock.NewStringResponse(http.StatusOK, `{
					"data": {
						"removeUserFromWorkspace": {
							"workspace": {
								"id": "01j9x0yy00000000000000001w",
								"name": "Test Workspace",
								"alias": "test-workspace",
								"personal": false
							}
						}
					}
				}`), nil
			},
		)

		got, err := client.WorkspaceRepo.RemoveUserFromWorkspace(ctx, "01j9x0yy00000000000000001w", "01j9x0yy00000000000000000a")

		assert.NoError(t, err)
		assert.NotNil(t, got)
		assert.Equal(t, "Test Workspace", got.Name())
	})

	t.Run("error on invalid workspace ID in response", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		httpmock.RegisterResponder(
			"POST",
			"https://accounts.example.com/api/graphql",
			httpmock.NewStringResponder(http.StatusOK, `{
				"data": {
					"removeUserFromWorkspace": {
						"workspace": {
							"id": "invalid-id",
							"name": "Test Workspace",
							"alias": "test-workspace",
							"personal": false
						}
					}
				}
			}`),
		)

		got, err := client.WorkspaceRepo.RemoveUserFromWorkspace(ctx, "01j9x0yy00000000000000001w", "01j9x0yy00000000000000000a")

		assert.Error(t, err)
		assert.Nil(t, got)
	})

	t.Run("graphql error", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		httpmock.RegisterResponder(
			"POST",
			"https://accounts.example.com/api/graphql",
			httpmock.NewStringResponder(http.StatusOK, `{
				"errors": [
					{
						"message": "Cannot remove the only owner",
						"path": ["removeUserFromWorkspace"]
					}
				]
			}`),
		)

		got, err := client.WorkspaceRepo.RemoveUserFromWorkspace(ctx, "01j9x0yy00000000000000001w", "01j9x0yy00000000000000000a")

		assert.Error(t, err)
		assert.Nil(t, got)
	})
}

func TestWorkspaceRepo_UpdateUserOfWorkspace(t *testing.T) {
	ctx := context.Background()

	t.Run("successfully update user role to WRITER", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		httpmock.RegisterResponder("POST", "https://accounts.example.com/api/graphql",
			func(req *http.Request) (*http.Response, error) {
				bodyBytes, _ := io.ReadAll(req.Body)
				fmt.Printf("\n--- UpdateUserOfWorkspace GraphQL Request ---\n%s\n", string(bodyBytes))
				req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				return httpmock.NewStringResponse(http.StatusOK, `{
					"data": {
						"updateUserOfWorkspace": {
							"workspace": {
								"id": "01j9x0yy00000000000000001w",
								"name": "Test Workspace",
								"alias": "test-workspace",
								"personal": false
							}
						}
					}
				}`), nil
			},
		)

		input := accountsGqlWorkspace.UpdateUserOfWorkspaceInput{
			WorkspaceID: "01j9x0yy00000000000000001w",
			UserID:      "01j9x0yy00000000000000000a",
			Role:        "WRITER",
		}

		got, err := client.WorkspaceRepo.UpdateUserOfWorkspace(ctx, input)

		assert.NoError(t, err)
		assert.NotNil(t, got)
		assert.Equal(t, "Test Workspace", got.Name())
	})

	t.Run("successfully update user role to OWNER", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		httpmock.RegisterResponder("POST", "https://accounts.example.com/api/graphql",
			func(req *http.Request) (*http.Response, error) {
				bodyBytes, _ := io.ReadAll(req.Body)
				fmt.Printf("\n--- UpdateUserOfWorkspace Owner GraphQL Request ---\n%s\n", string(bodyBytes))
				req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				return httpmock.NewStringResponse(http.StatusOK, `{
					"data": {
						"updateUserOfWorkspace": {
							"workspace": {
								"id": "01j9x0yy00000000000000001w",
								"name": "Test Workspace",
								"alias": "test-workspace",
								"personal": false
							}
						}
					}
				}`), nil
			},
		)

		input := accountsGqlWorkspace.UpdateUserOfWorkspaceInput{
			WorkspaceID: "01j9x0yy00000000000000001w",
			UserID:      "01j9x0yy00000000000000000a",
			Role:        "OWNER",
		}

		got, err := client.WorkspaceRepo.UpdateUserOfWorkspace(ctx, input)

		assert.NoError(t, err)
		assert.NotNil(t, got)
	})

	t.Run("error on invalid workspace ID in response", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		httpmock.RegisterResponder(
			"POST",
			"https://accounts.example.com/api/graphql",
			httpmock.NewStringResponder(http.StatusOK, `{
				"data": {
					"updateUserOfWorkspace": {
						"workspace": {
							"id": "invalid-id",
							"name": "Test Workspace",
							"alias": "test-workspace",
							"personal": false
						}
					}
				}
			}`),
		)

		input := accountsGqlWorkspace.UpdateUserOfWorkspaceInput{
			WorkspaceID: "01j9x0yy00000000000000001w",
			UserID:      "01j9x0yy00000000000000000a",
			Role:        "WRITER",
		}

		got, err := client.WorkspaceRepo.UpdateUserOfWorkspace(ctx, input)

		assert.Error(t, err)
		assert.Nil(t, got)
	})

	t.Run("graphql error", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		httpmock.RegisterResponder(
			"POST",
			"https://accounts.example.com/api/graphql",
			httpmock.NewStringResponder(http.StatusOK, `{
				"errors": [
					{
						"message": "User is not a member of this workspace",
						"path": ["updateUserOfWorkspace"]
					}
				]
			}`),
		)

		input := accountsGqlWorkspace.UpdateUserOfWorkspaceInput{
			WorkspaceID: "01j9x0yy00000000000000001w",
			UserID:      "01j9x0yy00000000000000000a",
			Role:        "WRITER",
		}

		got, err := client.WorkspaceRepo.UpdateUserOfWorkspace(ctx, input)

		assert.Error(t, err)
		assert.Nil(t, got)
	})
}
