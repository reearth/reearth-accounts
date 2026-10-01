package httpmodel

import "github.com/reearth/reearth-accounts/server/pkg/workspace"

// GenerateScimTokenResponse is the response for POST /api/workspaces/:id/scim/token.
type GenerateScimTokenResponse struct {
	Token   string `json:"token"`
	Warning string `json:"warning"`
}

// ScimConfigResponse is the response for GET and PUT /api/workspaces/:id/scim/config.
type ScimConfigResponse struct {
	Enabled          bool              `json:"enabled"`
	GroupRoleMapping map[string]string `json:"group_role_mapping"`
	ScimBaseURL      string            `json:"scim_base_url"`
	TokenIssued      bool              `json:"token_issued"`
}

// UpdateScimConfigRequest is the request body for PUT /api/workspaces/:id/scim/config.
type UpdateScimConfigRequest struct {
	Enabled          bool              `json:"enabled"`
	GroupRoleMapping map[string]string `json:"group_role_mapping"`
}

// NewScimConfigResponse converts a domain ScimConfig to a ScimConfigResponse.
// The token hash is masked: token_issued is true when a hash exists, but the raw hash is never included.
func NewScimConfigResponse(cfg *workspace.ScimConfig, scimBaseURL string) ScimConfigResponse {
	resp := ScimConfigResponse{
		GroupRoleMapping: map[string]string{},
		ScimBaseURL:      scimBaseURL,
	}
	if cfg == nil {
		return resp
	}
	resp.Enabled = cfg.Enabled()
	resp.TokenIssued = cfg.TokenHash() != ""
	for k, v := range cfg.GroupRoleMapping() {
		resp.GroupRoleMapping[k] = string(v)
	}
	return resp
}
