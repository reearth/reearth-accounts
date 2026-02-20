package cerbos

import (
	"context"
	"testing"

	"github.com/hasura/go-graphql-client"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

func TestCerbosRepo_CheckPermission(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	client := graphql.NewClient("https://example.com/graphql", nil)
	repo := NewRepo(client)
	ctx := context.Background()

	tests := []struct {
		name           string
		param          CheckPermissionParam
		mockResponse   string
		expectedResult *CheckPermissionResult
		expectError    bool
	}{
		{
			name: "permission allowed without workspace",
			param: CheckPermissionParam{
				Service:  "cms",
				Resource: "project",
				Action:   "read",
			},
			mockResponse: `{
				"data": {
					"checkPermission": {
						"allowed": true
					}
				}
			}`,
			expectedResult: &CheckPermissionResult{Allowed: true},
			expectError:    false,
		},
		{
			name: "permission denied without workspace",
			param: CheckPermissionParam{
				Service:  "cms",
				Resource: "project",
				Action:   "delete",
			},
			mockResponse: `{
				"data": {
					"checkPermission": {
						"allowed": false
					}
				}
			}`,
			expectedResult: &CheckPermissionResult{Allowed: false},
			expectError:    false,
		},
		{
			name: "permission allowed with workspace",
			param: CheckPermissionParam{
				Service:        "cms",
				Resource:       "model",
				Action:         "write",
				WorkspaceAlias: stringPtr("my-workspace"),
			},
			mockResponse: `{
				"data": {
					"checkPermission": {
						"allowed": true
					}
				}
			}`,
			expectedResult: &CheckPermissionResult{Allowed: true},
			expectError:    false,
		},
		{
			name: "permission denied with workspace",
			param: CheckPermissionParam{
				Service:        "cms",
				Resource:       "integration",
				Action:         "create",
				WorkspaceAlias: stringPtr("my-workspace"),
			},
			mockResponse: `{
				"data": {
					"checkPermission": {
						"allowed": false
					}
				}
			}`,
			expectedResult: &CheckPermissionResult{Allowed: false},
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpmock.Reset()
			httpmock.RegisterResponder(
				"POST",
				"https://example.com/graphql",
				httpmock.NewStringResponder(200, tt.mockResponse),
			)

			result, err := repo.CheckPermission(ctx, tt.param)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectedResult.Allowed, result.Allowed)
			}
		})
	}
}

func stringPtr(s string) *string {
	return &s
}
