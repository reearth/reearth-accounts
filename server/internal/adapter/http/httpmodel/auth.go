package httpmodel

import "github.com/reearth/reearth-accounts/server/internal/adapter"

// AuthConfigResponse mirrors the GraphQL AuthConfig type.
type AuthConfigResponse struct {
	Auth0Domain   *string `json:"auth0_domain,omitempty"`
	Auth0Audience *string `json:"auth0_audience,omitempty"`
	Auth0ClientID *string `json:"auth0_client_id,omitempty"`
	AuthProvider  *string `json:"auth_provider,omitempty"`
}

// NewAuthConfigResponse converts the adapter AuthConfigData to a REST response.
func NewAuthConfigResponse(d *adapter.AuthConfigData) *AuthConfigResponse {
	if d == nil {
		return &AuthConfigResponse{}
	}
	return &AuthConfigResponse{
		Auth0Domain:   d.Auth0Domain,
		Auth0Audience: d.Auth0Audience,
		Auth0ClientID: d.Auth0ClientID,
		AuthProvider:  d.AuthProvider,
	}
}

// CheckPermissionRequest mirrors checkPermission input.
type CheckPermissionRequest struct {
	Service        string  `json:"service" validate:"required"`
	Resource       string  `json:"resource" validate:"required"`
	Action         string  `json:"action" validate:"required"`
	WorkspaceAlias *string `json:"workspace_alias,omitempty"`
}

// CheckPermissionResponse mirrors checkPermission payload.
type CheckPermissionResponse struct {
	Allowed bool `json:"allowed"`
}
