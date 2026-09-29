package workspace

import "github.com/reearth/reearth-accounts/server/pkg/role"

// ScimConfig holds the per-workspace SCIM 2.0 provisioning settings.
// TokenHash stores a bcrypt hash of the bearer token; the plaintext is returned
// only once at generation time and is not stored.
type ScimConfig struct {
	enabled          bool
	groupRoleMapping map[string]role.RoleType
	tokenHash        string
}

func NewScimConfig() *ScimConfig {
	return &ScimConfig{
		groupRoleMapping: make(map[string]role.RoleType),
	}
}

func (s *ScimConfig) Clone() *ScimConfig {
	if s == nil {
		return nil
	}
	m := make(map[string]role.RoleType, len(s.groupRoleMapping))
	for k, v := range s.groupRoleMapping {
		m[k] = v
	}
	return &ScimConfig{
		enabled:          s.enabled,
		groupRoleMapping: m,
		tokenHash:        s.tokenHash,
	}
}

func (s *ScimConfig) Enabled() bool {
	if s == nil {
		return false
	}
	return s.enabled
}

func (s *ScimConfig) GroupRoleMapping() map[string]role.RoleType {
	if s == nil {
		return nil
	}
	m := make(map[string]role.RoleType, len(s.groupRoleMapping))
	for k, v := range s.groupRoleMapping {
		m[k] = v
	}
	return m
}

func (s *ScimConfig) SetEnabled(v bool) {
	s.enabled = v
}

func (s *ScimConfig) SetGroupRoleMapping(m map[string]role.RoleType) {
	if m == nil {
		s.groupRoleMapping = make(map[string]role.RoleType)
		return
	}
	nm := make(map[string]role.RoleType, len(m))
	for k, v := range m {
		nm[k] = v
	}
	s.groupRoleMapping = nm
}

func (s *ScimConfig) SetTokenHash(hash string) {
	s.tokenHash = hash
}

func (s *ScimConfig) TokenHash() string {
	if s == nil {
		return ""
	}
	return s.tokenHash
}
