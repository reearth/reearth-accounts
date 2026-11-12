package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/appx"
	"github.com/reearth/reearthx/log"
	"github.com/reearth/reearthx/rerror"
)

const (
	grantTypeClientCredentials = "client-credentials"
	clientSubjectSuffix        = "@clients"
)

var errServiceCallerRejected = errors.New("service caller rejected")

type serviceCallerKey struct {
	issuer   string
	clientID string
}

// ServiceCallers maps an (issuer, client ID) pair to the integration it acts as
// (ADR-0002 decision 3). Client IDs are unique only within one identity provider,
// so the issuer is part of the key.
type ServiceCallers map[serviceCallerKey]id.IntegrationID

type serviceCallerEntry struct {
	Issuer        string `json:"issuer"`
	ClientID      string `json:"client_id"`
	IntegrationID string `json:"integration_id"`
}

// Decode parses REEARTH_ACCOUNTS_SERVICE_CALLERS, a JSON array of
// {"issuer","client_id","integration_id"} objects.
func (s *ServiceCallers) Decode(value string) error {
	if strings.TrimSpace(value) == "" {
		*s = nil
		return nil
	}
	var entries []serviceCallerEntry
	if err := json.Unmarshal([]byte(value), &entries); err != nil {
		return fmt.Errorf("invalid service callers json: %w", err)
	}
	res := make(ServiceCallers, len(entries))
	for _, e := range entries {
		if e.Issuer == "" || e.ClientID == "" {
			return fmt.Errorf("service caller needs issuer and client_id: %+v", e)
		}
		iid, err := id.IntegrationIDFrom(e.IntegrationID)
		if err != nil {
			return fmt.Errorf("service caller %q: invalid integration_id %q", e.ClientID, e.IntegrationID)
		}
		k := serviceCallerKey{issuer: e.Issuer, clientID: e.ClientID}
		if _, dup := res[k]; dup {
			return fmt.Errorf("service caller %q is registered twice for issuer %q", e.ClientID, e.Issuer)
		}
		res[k] = iid
	}
	*s = res
	return nil
}

// checkIssuers fails when a caller names an issuer accounts does not validate
// tokens for, since such a caller could never authenticate.
func (s ServiceCallers) checkIssuers(providers []appx.JWTProvider) error {
	known := make(map[string]struct{}, len(providers))
	for _, p := range providers {
		known[p.ISS] = struct{}{}
	}
	for k := range s {
		if _, ok := known[k.issuer]; !ok {
			return fmt.Errorf("service caller %q: issuer %q is not a configured auth provider", k.clientID, k.issuer)
		}
	}
	return nil
}

type ServiceCaller struct {
	ClientID    string
	Integration id.IntegrationID
}

// tokenClaims holds the raw claims of a token appx has already verified. Claims
// are read leniently: a user token may carry non-string values (Auth0 can send
// gty as an array) and must still reach the user path.
type tokenClaims map[string]json.RawMessage

// str returns the claim as a string; ok is false when it is present but not a string.
func (c tokenClaims) str(name string) (v string, ok bool) {
	raw, present := c[name]
	if !present {
		return "", true
	}
	return v, json.Unmarshal(raw, &v) == nil
}

// resolve classifies a token appx has already validated (issuer, signature,
// audience, expiry). It returns (nil, nil) for a token that does not claim to be a
// client credentials token, so the user path handles it unchanged. A token that
// claims to be one is accepted only when it carries no user and its client is
// registered for its exact issuer; anything else is errServiceCallerRejected
// (ADR-0002 decision 2).
func (s ServiceCallers) resolve(ai appx.AuthInfo) (*ServiceCaller, error) {
	if len(s) == 0 || ai.Token == "" {
		return nil, nil
	}
	c, err := decodeTokenClaims(ai.Token)
	if err != nil {
		return nil, nil
	}
	sub, subOK := c.str("sub")
	gty, gtyOK := c.str("gty")
	if !(gtyOK && gty == grantTypeClientCredentials) && !(subOK && strings.HasSuffix(sub, clientSubjectSuffix)) {
		return nil, nil
	}

	azp, azpOK := c.str("azp")
	clientIDClaim, cidOK := c.str("client_id")
	email, emailOK := c.str("email")
	if !subOK || !gtyOK || !azpOK || !cidOK || !emailOK {
		return nil, errServiceCallerRejected
	}
	clientID := azp
	if clientIDClaim != "" {
		if clientID != "" && clientID != clientIDClaim {
			return nil, errServiceCallerRejected
		}
		clientID = clientIDClaim
	}
	if clientID == "" {
		return nil, errServiceCallerRejected
	}
	if gty != "" && gty != grantTypeClientCredentials {
		return nil, errServiceCallerRejected
	}
	if sub != clientID+clientSubjectSuffix && sub != clientID {
		return nil, errServiceCallerRejected
	}
	if email != "" || ai.Email != "" {
		return nil, errServiceCallerRejected
	}

	iid, ok := s[serviceCallerKey{issuer: ai.Iss, clientID: clientID}]
	if !ok {
		return nil, errServiceCallerRejected
	}
	return &ServiceCaller{ClientID: clientID, Integration: iid}, nil
}

func decodeTokenClaims(token string) (tokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("not a JWS compact token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	var c tokenClaims
	err = json.Unmarshal(payload, &c)
	return c, err
}

// generateServiceOperator builds the operator for a service caller: no user, and
// the workspaces where its integration is a member, by role (ADR-0002 decision 4).
func generateServiceOperator(ctx context.Context, cfg *ServerConfig, sc *ServiceCaller) (*workspace.Operator, error) {
	iid := sc.Integration
	ws, err := cfg.Repos.Workspace.FindByIntegration(ctx, iid)
	if err != nil && !errors.Is(err, rerror.ErrNotFound) {
		return nil, err
	}
	return &workspace.Operator{
		Integration:            &iid,
		ReadableWorkspaces:     ws.FilterByIntegrationRole(iid, role.RoleReader).IDs(),
		WritableWorkspaces:     ws.FilterByIntegrationRole(iid, role.RoleWriter).IDs(),
		MaintainableWorkspaces: ws.FilterByIntegrationRole(iid, role.RoleMaintainer).IDs(),
		OwningWorkspaces:       ws.FilterByIntegrationRole(iid, role.RoleOwner).IDs(),
	}, nil
}

// resolveServiceOperator returns (nil, nil) when the request is not a service
// call. It logs the client and integration of every accepted call (ADR-0002
// decision 6).
func resolveServiceOperator(ctx context.Context, cfg *ServerConfig, ai *appx.AuthInfo) (*workspace.Operator, error) {
	if ai == nil {
		return nil, nil
	}
	sc, err := cfg.Config.ServiceCallers.resolve(*ai)
	if err != nil {
		log.Warnfc(ctx, "auth: rejected client credentials token iss=%s sub=%s", ai.Iss, ai.Sub)
		return nil, err
	}
	if sc == nil {
		return nil, nil
	}
	op, err := generateServiceOperator(ctx, cfg, sc)
	if err != nil {
		return nil, err
	}
	log.Infofc(ctx, "auth: service caller client_id=%s integration=%s", sc.ClientID, sc.Integration)
	return op, nil
}
