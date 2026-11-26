package http

import (
	"maps"

	"github.com/reearth/reearth-accounts/server/internal/adapter"
)

// clientRoutes is the complete list of REST routes a client token may reach,
// keyed "METHOD /path" as echo reports the route pattern. A route that is not
// here answers 403 to every client token, so a route added later stays closed
// until someone lists it. The SCIM token and config routes are absent on
// purpose: a client must not mint a workspace credential.
var clientRoutes = map[string]adapter.ClientAccess{
	"GET /api/auth/config": adapter.AnonymousClients | adapter.IntegrationClients,

	"GET /api/users/me": adapter.UserClients,
	// The interactor scopes it to the subject, pinned by TestWorkspace_ReadsForAClientActingForAUserAreScoped and TestActAsUser_ReadList.
	"GET /api/workspaces": adapter.IntegrationClients | adapter.UserClients,

	"GET /api/workspaces/:id":                                 adapter.IntegrationClients,
	"PATCH /api/workspaces/:id":                               adapter.IntegrationClients,
	"DELETE /api/workspaces/:id":                              adapter.IntegrationClients,
	"POST /api/workspaces/:id/deactivate":                     adapter.IntegrationClients,
	"POST /api/workspaces/:id/restore":                        adapter.IntegrationClients,
	"PATCH /api/workspaces/:id/members/:user_id":              adapter.IntegrationClients,
	"DELETE /api/workspaces/:id/members/:user_id":             adapter.IntegrationClients,
	"DELETE /api/workspaces/:id/members":                      adapter.IntegrationClients,
	"PATCH /api/workspaces/:id/integrations/:integration_id":  adapter.IntegrationClients,
	"DELETE /api/workspaces/:id/integrations/:integration_id": adapter.IntegrationClients,
	"DELETE /api/workspaces/:id/integrations":                 adapter.IntegrationClients,
	"PATCH /api/service/workspaces/:id/members/:user_id":      adapter.IntegrationClients,
}

// ClientRoutes returns a copy of the REST client table.
func ClientRoutes() map[string]adapter.ClientAccess {
	return maps.Clone(clientRoutes)
}
