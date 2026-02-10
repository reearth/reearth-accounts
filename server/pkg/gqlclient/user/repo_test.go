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
	"github.com/reearth/reearth-accounts/server/pkg/gqlclient/user"
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

func TestUserRepo_UpdateMe(t *testing.T) {
	ctx := context.Background()

	t.Run("successfully update with all fields", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		name := "Updated Name"
		email := "updated@example.com"
		lang := "ja"
		theme := "DARK"
		password := "newpassword123"
		passwordConfirmation := "newpassword123"

		httpmock.RegisterResponder("POST", "https://accounts.example.com/api/graphql",
			func(req *http.Request) (*http.Response, error) {
				bodyBytes, _ := io.ReadAll(req.Body)
				fmt.Printf("\n--- UpdateMe GraphQL Request ---\n%s\n", string(bodyBytes))
				req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				return httpmock.NewStringResponse(http.StatusOK, `{
					"data": {
						"updateMe": {
							"me": {
								"id": "01j9x0yy00000000000000000a",
								"name": "Updated Name",
								"alias": "testuser",
								"email": "updated@example.com",
								"myWorkspaceId": "01j9x0yy00000000000000001a",
								"host": "",
								"auths": ["auth0|123456"],
								"metadata": {
									"photoURL": "https://example.com/photo.jpg",
									"description": "Test description",
									"website": "https://example.com",
									"lang": "ja",
									"theme": "DARK"
								}
							}
						}
					}
				}`), nil
			},
		)

		input := user.UpdateMeInput{
			Name:                 &name,
			Email:                &email,
			Lang:                 &lang,
			Theme:                &theme,
			Password:             &password,
			PasswordConfirmation: &passwordConfirmation,
		}

		got, err := client.UserRepo.UpdateMe(ctx, input)

		assert.NoError(t, err)
		assert.NotNil(t, got)
		assert.Equal(t, "Updated Name", got.Name())
		assert.Equal(t, "updated@example.com", got.Email())
		assert.Equal(t, "ja", got.Metadata().Lang().String())
		assert.Equal(t, "dark", string(got.Metadata().Theme()))
	})

	t.Run("successfully update with partial fields", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		name := "Partial Update"

		httpmock.RegisterResponder("POST", "https://accounts.example.com/api/graphql",
			func(req *http.Request) (*http.Response, error) {
				bodyBytes, _ := io.ReadAll(req.Body)
				fmt.Printf("\n--- UpdateMe Partial GraphQL Request ---\n%s\n", string(bodyBytes))
				req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				return httpmock.NewStringResponse(http.StatusOK, `{
					"data": {
						"updateMe": {
							"me": {
								"id": "01j9x0yy00000000000000000a",
								"name": "Partial Update",
								"alias": "testuser",
								"email": "test@example.com",
								"myWorkspaceId": "01j9x0yy00000000000000001a",
								"host": "",
								"auths": ["auth0|123456"],
								"metadata": {
									"photoURL": "",
									"description": "",
									"website": "",
									"lang": "en",
									"theme": "light"
								}
							}
						}
					}
				}`), nil
			},
		)

		input := user.UpdateMeInput{
			Name: &name,
		}

		got, err := client.UserRepo.UpdateMe(ctx, input)

		assert.NoError(t, err)
		assert.NotNil(t, got)
		assert.Equal(t, "Partial Update", got.Name())
	})

	t.Run("error on invalid user ID", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		name := "Test"

		httpmock.RegisterResponder(
			"POST",
			"https://accounts.example.com/api/graphql",
			httpmock.NewStringResponder(http.StatusOK, `{
				"data": {
					"updateMe": {
						"me": {
							"id": "invalid-id",
							"name": "Test",
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
				}
			}`),
		)

		input := user.UpdateMeInput{
			Name: &name,
		}

		got, err := client.UserRepo.UpdateMe(ctx, input)

		assert.Error(t, err)
		assert.Nil(t, got)
	})
}

func TestUserRepo_RemoveMyAuth(t *testing.T) {
	ctx := context.Background()

	t.Run("successfully remove auth", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		httpmock.RegisterResponder("POST", "https://accounts.example.com/api/graphql",
			func(req *http.Request) (*http.Response, error) {
				bodyBytes, _ := io.ReadAll(req.Body)
				fmt.Printf("\n--- RemoveMyAuth GraphQL Request ---\n%s\n", string(bodyBytes))
				req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				return httpmock.NewStringResponse(http.StatusOK, `{
					"data": {
						"removeMyAuth": {
							"me": {
								"id": "01j9x0yy00000000000000000a",
								"name": "Test User",
								"alias": "testuser",
								"email": "test@example.com",
								"myWorkspaceId": "01j9x0yy00000000000000001a",
								"host": "",
								"auths": ["auth0|654321"],
								"metadata": {
									"photoURL": "https://example.com/photo.jpg",
									"description": "Test description",
									"website": "https://example.com",
									"lang": "en",
									"theme": "light"
								}
							}
						}
					}
				}`), nil
			},
		)

		got, err := client.UserRepo.RemoveMyAuth(ctx, "auth0|123456")

		assert.NoError(t, err)
		assert.NotNil(t, got)
		assert.Equal(t, "Test User", got.Name())
		assert.Equal(t, "testuser", got.Alias())
		assert.Equal(t, "test@example.com", got.Email())
		assert.Len(t, got.Auths(), 1)
		assert.Equal(t, "auth0|654321", got.Auths()[0].String())
	})

	t.Run("error on invalid user ID", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		httpmock.RegisterResponder(
			"POST",
			"https://accounts.example.com/api/graphql",
			httpmock.NewStringResponder(http.StatusOK, `{
				"data": {
					"removeMyAuth": {
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
				}
			}`),
		)

		got, err := client.UserRepo.RemoveMyAuth(ctx, "auth0|123456")

		assert.Error(t, err)
		assert.Nil(t, got)
	})

	t.Run("error on invalid workspace ID", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		httpmock.RegisterResponder(
			"POST",
			"https://accounts.example.com/api/graphql",
			httpmock.NewStringResponder(http.StatusOK, `{
				"data": {
					"removeMyAuth": {
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
				}
			}`),
		)

		got, err := client.UserRepo.RemoveMyAuth(ctx, "auth0|123456")

		assert.Error(t, err)
		assert.Nil(t, got)
	})
}

func TestUserRepo_DeleteMe(t *testing.T) {
	ctx := context.Background()

	t.Run("successfully delete me", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		httpmock.RegisterResponder("POST", "https://accounts.example.com/api/graphql",
			func(req *http.Request) (*http.Response, error) {
				bodyBytes, _ := io.ReadAll(req.Body)
				fmt.Printf("\n--- DeleteMe GraphQL Request ---\n%s\n", string(bodyBytes))
				req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				return httpmock.NewStringResponse(http.StatusOK, `{
					"data": {
						"deleteMe": {
							"userId": "01j9x0yy00000000000000000a"
						}
					}
				}`), nil
			},
		)

		err := client.UserRepo.DeleteMe(ctx, "01j9x0yy00000000000000000a")

		assert.NoError(t, err)
	})

	t.Run("error from server", func(t *testing.T) {
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
						"message": "User not found"
					}
				]
			}`),
		)

		err := client.UserRepo.DeleteMe(ctx, "01j9x0yy00000000000000000a")

		assert.Error(t, err)
	})

	t.Run("network error", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		httpmock.RegisterResponder(
			"POST",
			"https://accounts.example.com/api/graphql",
			httpmock.NewStringResponder(http.StatusInternalServerError, "Internal Server Error"),
		)

		err := client.UserRepo.DeleteMe(ctx, "01j9x0yy00000000000000000a")

		assert.Error(t, err)
	})
}

func TestUserRepo_FindByAlias(t *testing.T) {
	ctx := context.Background()

	t.Run("successfully find user by alias using findUserByAlias query", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		// Assert that the request uses findUserByAlias query with $alias variable
		httpmock.RegisterResponder("POST", "https://accounts.example.com/api/graphql",
			func(req *http.Request) (*http.Response, error) {
				bodyBytes, _ := io.ReadAll(req.Body)
				body := string(bodyBytes)
				fmt.Printf("\n--- FindByAlias GraphQL Request ---\n%s\n", body)

				// Verify the query uses findUserByAlias with $alias parameter
				assert.Contains(t, body, "findUserByAlias")
				assert.Contains(t, body, "$alias")
				assert.NotContains(t, body, "userByNameOrEmail", "FindByAlias should not use userByNameOrEmail query")

				req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				return httpmock.NewStringResponse(http.StatusOK, `{
					"data": {
						"findUserByAlias": {
							"id": "01j9x0yy00000000000000000a",
							"name": "Test User",
							"alias": "testuser",
							"email": "test@example.com",
							"workspace": "01j9x0yy00000000000000001a",
							"host": "",
							"auths": ["auth0|123456"]
						}
					}
				}`), nil
			},
		)

		got, err := client.UserRepo.FindByAlias(ctx, "testuser")

		assert.NoError(t, err)
		assert.NotNil(t, got)
		assert.Equal(t, "Test User", got.Name())
		assert.Equal(t, "testuser", got.Alias())
		assert.Equal(t, "test@example.com", got.Email())
	})

	t.Run("user not found by alias", func(t *testing.T) {
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
						"message": "input: findUserByAlias not found"
					}
				]
			}`),
		)

		got, err := client.UserRepo.FindByAlias(ctx, "nonexistent")

		assert.Error(t, err)
		assert.Nil(t, got)
	})

	t.Run("invalid user ID in alias lookup", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		httpmock.RegisterResponder(
			"POST",
			"https://accounts.example.com/api/graphql",
			httpmock.NewStringResponder(http.StatusOK, `{
				"data": {
					"findUserByAlias": {
						"id": "invalid-id",
						"name": "Test User",
						"alias": "testuser",
						"email": "test@example.com",
						"workspace": "01j9x0yy00000000000000001a",
						"host": "",
						"auths": []
					}
				}
			}`),
		)

		got, err := client.UserRepo.FindByAlias(ctx, "testuser")

		assert.Error(t, err)
		assert.Nil(t, got)
	})
}

func TestUserRepo_FindByNameOrEmail(t *testing.T) {
	ctx := context.Background()

	t.Run("successfully find user by name or email using userByNameOrEmail query", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		// Assert that the request uses userByNameOrEmail query with $nameOrEmail variable
		httpmock.RegisterResponder("POST", "https://accounts.example.com/api/graphql",
			func(req *http.Request) (*http.Response, error) {
				bodyBytes, _ := io.ReadAll(req.Body)
				body := string(bodyBytes)
				fmt.Printf("\n--- FindByNameOrEmail GraphQL Request ---\n%s\n", body)

				// Verify the query uses userByNameOrEmail with $nameOrEmail parameter
				assert.Contains(t, body, "userByNameOrEmail")
				assert.Contains(t, body, "$nameOrEmail")
				assert.NotContains(t, body, "findUserByAlias", "FindByNameOrEmail should not use findUserByAlias query")

				req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				return httpmock.NewStringResponse(http.StatusOK, `{
					"data": {
						"userByNameOrEmail": {
							"id": "01j9x0yy00000000000000000a",
							"name": "Test User",
							"email": "test@example.com",
							"workspace": "01j9x0yy00000000000000001a",
							"host": "",
							"auths": ["auth0|123456"]
						}
					}
				}`), nil
			},
		)

		got, err := client.UserRepo.FindByNameOrEmail(ctx, "test@example.com")

		assert.NoError(t, err)
		assert.NotNil(t, got)
		assert.Equal(t, "Test User", got.Name())
		assert.Equal(t, "test@example.com", got.Email())
	})

	t.Run("successfully find by name using userByNameOrEmail query", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		httpmock.RegisterResponder("POST", "https://accounts.example.com/api/graphql",
			func(req *http.Request) (*http.Response, error) {
				bodyBytes, _ := io.ReadAll(req.Body)
				body := string(bodyBytes)

				// Verify the query uses userByNameOrEmail (handles both name and email)
				assert.Contains(t, body, "userByNameOrEmail")
				assert.Contains(t, body, "$nameOrEmail")

				req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				return httpmock.NewStringResponse(http.StatusOK, `{
					"data": {
						"userByNameOrEmail": {
							"id": "01j9x0yy00000000000000000a",
							"name": "Test User",
							"email": "test@example.com",
							"workspace": "01j9x0yy00000000000000001a",
							"host": "",
							"auths": ["auth0|123456"]
						}
					}
				}`), nil
			},
		)

		got, err := client.UserRepo.FindByNameOrEmail(ctx, "Test User")

		assert.NoError(t, err)
		assert.NotNil(t, got)
		assert.Equal(t, "Test User", got.Name())
	})

	t.Run("user not found by name or email", func(t *testing.T) {
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
						"message": "input: userByNameOrEmail not found"
					}
				]
			}`),
		)

		got, err := client.UserRepo.FindByNameOrEmail(ctx, "nonexistent@example.com")

		assert.Error(t, err)
		assert.Nil(t, got)
	})

	t.Run("invalid user ID in name or email lookup", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		transport := httpmock.DefaultTransport
		client := gqlclient.NewClient("https://accounts.example.com", 30, transport)

		httpmock.RegisterResponder(
			"POST",
			"https://accounts.example.com/api/graphql",
			httpmock.NewStringResponder(http.StatusOK, `{
				"data": {
					"userByNameOrEmail": {
						"id": "invalid-id",
						"name": "Test User",
						"email": "test@example.com",
						"workspace": "01j9x0yy00000000000000001a",
						"host": "",
						"auths": []
					}
				}
			}`),
		)

		got, err := client.UserRepo.FindByNameOrEmail(ctx, "test@example.com")

		assert.Error(t, err)
		assert.Nil(t, got)
	})
}
