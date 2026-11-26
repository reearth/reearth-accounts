package adapter

import (
	"strings"

	"github.com/reearth/reearth-accounts/server/pkg/workspace"
)

// ClientAccess is the set of client subject kinds a REST route or GraphQL root
// field admits (ADR-0002 decision 7.4). The zero value admits no client token,
// which is the default for every route and field.
type ClientAccess uint8

const (
	AnonymousClients ClientAccess = 1 << iota
	IntegrationClients
	UserClients
)

func (a ClientAccess) Admits(k workspace.SubjectKind) bool {
	switch k {
	case workspace.SubjectNone:
		return a&AnonymousClients != 0
	case workspace.SubjectIntegration:
		return a&IntegrationClients != 0
	case workspace.SubjectUser:
		return a&UserClients != 0
	default:
		return false
	}
}

func (a ClientAccess) String() string {
	var kinds []string
	for _, k := range []struct {
		bit  ClientAccess
		name string
	}{{AnonymousClients, "anonymous"}, {IntegrationClients, "integration"}, {UserClients, "user"}} {
		if a&k.bit != 0 {
			kinds = append(kinds, k.name)
		}
	}
	if len(kinds) == 0 {
		return "none"
	}
	return strings.Join(kinds, ",")
}
