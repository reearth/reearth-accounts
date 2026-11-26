package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/pkg/applog"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/appx"
	"github.com/reearth/reearthx/rerror"
)

const (
	grantTypeClientCredentials = "client-credentials"
	clientSubjectSuffix        = "@clients"

	// ActAsHeader names the subject a client call acts for (ADR-0002 decision 5).
	ActAsHeader = "X-Reearth-Act-As"
)

type serviceCallerKey struct {
	issuer   string
	clientID string
}

// ActAs is the set of subject kinds a client may act for (ADR-0002 decision 3).
type ActAs uint8

const (
	ActAsIntegration ActAs = 1 << iota
	ActAsUser
)

var actAsNames = map[string]ActAs{"integration": ActAsIntegration, "user": ActAsUser}

// Allows reports whether a client may act for kind. A call that acts for no
// one is anonymous and always allowed (decision 7.1).
func (a ActAs) Allows(k workspace.SubjectKind) bool {
	switch k {
	case workspace.SubjectNone:
		return true
	case workspace.SubjectIntegration:
		return a&ActAsIntegration != 0
	case workspace.SubjectUser:
		return a&ActAsUser != 0
	default:
		return false
	}
}

// ServiceClient is one registry entry after parsing.
type ServiceClient struct {
	Issuer string
	workspace.Client
	ActAs ActAs
}

// ServiceCallers maps (issuer, client ID) to its registry entry. Client IDs are
// unique only within one identity provider, so the issuer is part of the key.
type ServiceCallers map[serviceCallerKey]ServiceClient

type serviceCallerEntry struct {
	Issuer   string `json:"issuer"`
	ClientID string `json:"client_id"`
	Name     string `json:"name"`
	// A pointer, so that a missing key is an error and [] means "neither".
	ActAs *[]string `json:"act_as"`
}

var serviceCallerKeys = map[string]struct{}{"issuer": {}, "client_id": {}, "name": {}, "act_as": {}}

// The name is logged verbatim, so its charset is fixed.
var serviceCallerName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// Decode parses REEARTH_ACCOUNTS_SERVICE_CALLERS, a JSON array of
// {"issuer","client_id","name","act_as"} objects. Keys must match exactly and
// appear once, so a registry written for an older shape, or edited into an
// ambiguous one, stops startup instead of being misread.
func (s *ServiceCallers) Decode(value string) error {
	if strings.TrimSpace(value) == "" {
		*s = nil
		return nil
	}
	var raw []json.RawMessage
	if err := json.Unmarshal([]byte(value), &raw); err != nil {
		return fmt.Errorf("invalid service callers json: %w", err)
	}
	if raw == nil {
		return errors.New("invalid service callers json: the value must be an array")
	}

	res := make(ServiceCallers, len(raw))
	for i, r := range raw {
		c, err := decodeServiceCaller(r)
		if err != nil {
			return fmt.Errorf("service caller %d: %w", i+1, err)
		}
		k := serviceCallerKey{issuer: c.Issuer, clientID: c.ID}
		if _, dup := res[k]; dup {
			return fmt.Errorf("service caller %q is registered twice for issuer %q", c.ID, c.Issuer)
		}
		res[k] = c
	}
	*s = res
	return nil
}

// decodeServiceCaller walks the entry's tokens before decoding it, because
// encoding/json matches keys case-insensitively and lets a repeated key win.
func decodeServiceCaller(raw json.RawMessage) (ServiceClient, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return ServiceClient{}, errors.New("must be an object")
	}
	seen := make(map[string]struct{}, len(serviceCallerKeys))
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return ServiceClient{}, err
		}
		k, _ := t.(string)
		if k == "integration_id" {
			return ServiceClient{}, errors.New("integration_id is no longer supported: replace it with act_as and send X-Reearth-Act-As (ADR-0002 decision 3)")
		}
		if _, ok := serviceCallerKeys[k]; !ok {
			return ServiceClient{}, fmt.Errorf("unknown key %q", k)
		}
		if _, dup := seen[k]; dup {
			return ServiceClient{}, fmt.Errorf("key %q appears twice", k)
		}
		seen[k] = struct{}{}
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return ServiceClient{}, err
		}
	}
	var e serviceCallerEntry
	if err := json.Unmarshal(raw, &e); err != nil {
		return ServiceClient{}, err
	}
	return e.parse()
}

func (e serviceCallerEntry) parse() (ServiceClient, error) {
	if e.Issuer == "" || e.ClientID == "" || e.Name == "" {
		return ServiceClient{}, errors.New("issuer, client_id and name are required")
	}
	if !serviceCallerName.MatchString(e.Name) {
		return ServiceClient{}, fmt.Errorf("name %q must match %s", e.Name, serviceCallerName)
	}
	if e.ActAs == nil {
		return ServiceClient{}, errors.New(`act_as is required; use [] for a client that only makes anonymous calls`)
	}
	var actAs ActAs
	for _, v := range *e.ActAs {
		bit, ok := actAsNames[v]
		if !ok {
			return ServiceClient{}, fmt.Errorf(`act_as value %q must be "integration" or "user"`, v)
		}
		if actAs&bit != 0 {
			return ServiceClient{}, fmt.Errorf("act_as lists %q twice", v)
		}
		actAs |= bit
	}
	return ServiceClient{Issuer: e.Issuer, Client: workspace.Client{ID: e.ClientID, Name: e.Name}, ActAs: actAs}, nil
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

// rejectReason is logged. The raw header and the token never are.
type rejectReason string

const (
	reasonBadClaims             rejectReason = "bad_client_claims"
	reasonUnregistered          rejectReason = "unregistered_client"
	reasonActAsWithoutClient    rejectReason = "act_as_without_client_token"
	reasonMalformedActAs        rejectReason = "malformed_act_as"
	reasonKindNotAllowed        rejectReason = "act_as_kind_not_allowed"
	reasonActAsUserNotAvailable rejectReason = "act_as_user_not_available"
)

// clientRejected always becomes a 401.
type clientRejected struct {
	reason   rejectReason
	clientID string
}

func (e *clientRejected) Error() string { return "client call rejected: " + string(e.reason) }

func isClientRejected(err error) bool {
	var r *clientRejected
	return errors.As(err, &r)
}

// parseActAs parses one X-Reearth-Act-As value: exactly "integration:<id>" or
// "user:<id>", with a lower-case kind and an ID the id package accepts.
func parseActAs(v string) (workspace.Subject, error) {
	kind, raw, ok := strings.Cut(v, ":")
	if !ok || raw == "" || strings.Contains(raw, ":") {
		return workspace.Subject{}, errors.New("act-as must be <kind>:<id>")
	}
	switch kind {
	case "integration":
		iid, err := id.IntegrationIDFrom(raw)
		if err != nil {
			return workspace.Subject{}, errors.New("act-as has an invalid integration id")
		}
		return workspace.IntegrationSubject(iid), nil
	case "user":
		uid, err := id.UserIDFrom(raw)
		if err != nil {
			return workspace.Subject{}, errors.New("act-as has an invalid user id")
		}
		return workspace.UserSubject(uid), nil
	default:
		return workspace.Subject{}, errors.New("act-as has an unknown kind")
	}
}

type clientAuth struct {
	client  ServiceClient
	subject workspace.Subject
}

// authenticate decides, with no I/O, whether a request is a registered client
// call and what it acts for. actAs is every X-Reearth-Act-As value the request
// carries, so an empty or repeated header is refused rather than read as absent.
// It returns (nil, nil) when the user path should handle the request.
func (s ServiceCallers) authenticate(ai *appx.AuthInfo, actAs []string) (*clientAuth, error) {
	clientID, err := tokenClientID(ai)
	if err != nil {
		return nil, err
	}
	if clientID == "" {
		if len(actAs) > 0 {
			return nil, &clientRejected{reason: reasonActAsWithoutClient}
		}
		return nil, nil
	}
	reject := func(r rejectReason) error { return &clientRejected{reason: r, clientID: clientID} }

	c, ok := s[serviceCallerKey{issuer: ai.Iss, clientID: clientID}]
	if !ok {
		return nil, reject(reasonUnregistered)
	}
	var sub workspace.Subject
	switch len(actAs) {
	case 0:
	case 1:
		if sub, err = parseActAs(actAs[0]); err != nil {
			return nil, reject(reasonMalformedActAs)
		}
	default:
		return nil, reject(reasonMalformedActAs)
	}
	if !c.ActAs.Allows(sub.Kind()) {
		return nil, reject(reasonKindNotAllowed)
	}
	// Acting for a user (ADR-0002 decision 6) is not available yet, so a user
	// subject is refused here even when act_as allows it. Without a route gate,
	// an operator with both a user and a client would pass every user-only guard.
	if sub.Kind() == workspace.SubjectUser {
		return nil, reject(reasonActAsUserNotAvailable)
	}
	return &clientAuth{client: c, subject: sub}, nil
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

// tokenClientID classifies a token appx has already validated (issuer,
// signature, audience, expiry), whatever the registry holds. It returns "" for a
// token that does not claim to be a client credentials token, so the user path
// handles it unchanged. A token that claims to be one must carry no user
// (ADR-0002 decision 2).
func tokenClientID(ai *appx.AuthInfo) (string, error) {
	if ai == nil || ai.Token == "" {
		return "", nil
	}
	c, err := decodeTokenClaims(ai.Token)
	if err != nil {
		return "", nil
	}
	sub, subOK := c.str("sub")
	gty, gtyOK := c.str("gty")
	if !(gtyOK && gty == grantTypeClientCredentials) && !(subOK && strings.HasSuffix(sub, clientSubjectSuffix)) {
		return "", nil
	}

	bad := &clientRejected{reason: reasonBadClaims}
	azp, azpOK := c.str("azp")
	clientIDClaim, cidOK := c.str("client_id")
	email, emailOK := c.str("email")
	if !subOK || !gtyOK || !azpOK || !cidOK || !emailOK {
		return "", bad
	}
	clientID := azp
	if clientIDClaim != "" {
		if clientID != "" && clientID != clientIDClaim {
			return "", bad
		}
		clientID = clientIDClaim
	}
	if clientID == "" {
		return "", bad
	}
	bad.clientID = clientID
	if gty != "" && gty != grantTypeClientCredentials {
		return "", bad
	}
	if sub != clientID+clientSubjectSuffix && sub != clientID {
		return "", bad
	}
	if email != "" || ai.Email != "" {
		return "", bad
	}
	return clientID, nil
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

// generateIntegrationOperator builds the operator for a client acting for an
// integration: no user, and the workspaces where the integration is a member,
// by role.
func generateIntegrationOperator(ctx context.Context, cfg *ServerConfig, iid id.IntegrationID, c *workspace.Client) (*workspace.Operator, error) {
	ws, err := cfg.Repos.Workspace.FindByIntegration(ctx, iid)
	if err != nil && !errors.Is(err, rerror.ErrNotFound) {
		return nil, err
	}
	return &workspace.Operator{
		Integration:            &iid,
		Client:                 c,
		ReadableWorkspaces:     ws.FilterByIntegrationRole(iid, role.RoleReader).IDs(),
		WritableWorkspaces:     ws.FilterByIntegrationRole(iid, role.RoleWriter).IDs(),
		MaintainableWorkspaces: ws.FilterByIntegrationRole(iid, role.RoleMaintainer).IDs(),
		OwningWorkspaces:       ws.FilterByIntegrationRole(iid, role.RoleOwner).IDs(),
	}, nil
}

// resolveClientCall turns a request into a client call for both transports.
func resolveClientCall(ctx context.Context, cfg *ServerConfig, ai *appx.AuthInfo, actAs []string) (*adapter.ClientCall, error) {
	ca, err := cfg.Config.ServiceCallers.authenticate(ai, actAs)
	if err != nil {
		logRejection(ctx, ai, err)
		return nil, err
	}
	if ca == nil {
		return nil, nil
	}

	client := ca.client.Client
	cc := &adapter.ClientCall{Client: &client, Subject: ca.subject}
	if iid, ok := ca.subject.Integration(); ok {
		if cc.Operator, err = generateIntegrationOperator(ctx, cfg, iid, cc.Client); err != nil {
			return nil, err
		}
		cc.LogVerdict(ctx, true)
	}
	// An anonymous call is logged by the transport, which knows whether the
	// operation is public.
	return cc, nil
}

// callerClientID is the client ID a request's token names, for the request log
// when no client call is attached: a refused call, or a route with no auth.
func callerClientID(ai *appx.AuthInfo) string {
	cid, err := tokenClientID(ai)
	var r *clientRejected
	if errors.As(err, &r) {
		return r.clientID
	}
	return cid
}

func logRejection(ctx context.Context, ai *appx.AuthInfo, err error) {
	var r *clientRejected
	if !errors.As(err, &r) {
		return
	}
	fields := []applog.Field{applog.Reason(string(r.reason))}
	if ai != nil && ai.Iss != "" {
		fields = append(fields, applog.Issuer(ai.Iss))
	}
	if r.clientID != "" {
		fields = append(fields, applog.CallerClientID(r.clientID))
	}
	applog.Warn(ctx, "auth: rejected client call", fields...)
}
