package mongodoc

import (
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/sso"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/samber/lo"
)

type WorkspaceSSOConfigDocument struct {
	Auth0ConnectionID string   `json:"auth0_connection_id,omitempty" bson:"auth0_connection_id,omitempty"`
	Auth0OrgID        string   `json:"auth0_org_id,omitempty" bson:"auth0_org_id,omitempty"`
	ClientID          string   `json:"client_id,omitempty" bson:"client_id,omitempty"`
	ClientSecret      string   `json:"client_secret,omitempty" bson:"client_secret,omitempty"`
	ConnectionType    string   `json:"connection_type" bson:"connection_type"`
	DirectoryDomain   string   `json:"directory_domain,omitempty" bson:"directory_domain,omitempty"`
	EmailDomains      []string `json:"email_domains,omitempty" bson:"email_domains,omitempty"`
	Enabled           bool     `json:"enabled" bson:"enabled"`
	JITDefaultRole    string   `json:"jit_default_role,omitempty" bson:"jit_default_role,omitempty"`
	OIDCClientID      string   `json:"oidc_client_id,omitempty" bson:"oidc_client_id,omitempty"`
	OIDCClientSecret  string   `json:"oidc_client_secret,omitempty" bson:"oidc_client_secret,omitempty"`
	OIDCDiscoveryURL  string   `json:"oidc_discovery_url,omitempty" bson:"oidc_discovery_url,omitempty"`
	SAMLEntityID      string   `json:"saml_entity_id,omitempty" bszon:"saml_entity_id,omitempty"`
	SAMLMetadataURL   string   `json:"saml_metadata_url,omitempty" bson:"saml_metadata_url,omitempty"`
	SAMLSignInURL     string   `json:"saml_sign_in_url,omitempty" bson:"saml_sign_in_url,omitempty"`
	SAMLSignOutURL    string   `json:"saml_sign_out_url,omitempty" bson:"saml_sign_out_url,omitempty"`
	SAMLX509Cert      string   `json:"saml_x509_cert,omitempty" bson:"saml_x509_cert,omitempty"`
	Verified          bool     `json:"verified" bson:"verified"`
}

type WorkspaceMemberDocument struct {
	Role      string `json:"role" jsonschema:"description=Member role (owner, maintainer, writer, reader). Default: \"\""`
	InvitedBy string `json:"invitedby" jsonschema:"description=User ID of the inviter"`
	Disabled  bool   `json:"disabled" jsonschema:"description=Whether the member is disabled"`
}

type WorkspaceMetadataDocument struct {
	Description  string `json:"description" jsonschema:"description=Workspace description. Default: \"\""`
	Website      string `json:"website" jsonschema:"description=Workspace website URL. Default: \"\""`
	Location     string `json:"location" jsonschema:"description=Workspace location. Default: \"\""`
	BillingEmail string `json:"billingemail" jsonschema:"description=Billing email address. Default: \"\""`
	PhotoURL     string `json:"photourl" jsonschema:"description=Workspace photo URL. Default: \"\""`
}

type WorkspaceDocument struct {
	ID           string                             `json:"id" bson:"id" jsonschema:"required,description=Workspace ID (ULID format)"`
	Name         string                             `json:"name" bson:"name" jsonschema:"required,description=Workspace name"`
	Alias        string                             `json:"alias" bson:"alias" jsonschema:"required,description=Unique workspace handle/alias"`
	Email        string                             `json:"email" bson:"email" jsonschema:"required,description=Workspace contact email"`
	Metadata     WorkspaceMetadataDocument          `json:"metadata" bson:"metadata" jsonschema:"required,description=Extended workspace metadata"`
	Members      map[string]WorkspaceMemberDocument `json:"members" bson:"members" jsonschema:"required,description=Map of user ID to member document"`
	Integrations map[string]WorkspaceMemberDocument `json:"integrations" bson:"integrations" jsonschema:"description=Map of integration ID to member document. Default: {}"`
	MembersHash  string                             `json:"members_hash" bson:"members_hash,omitempty" jsonschema:"description=SHA256 hash of members and integrations for uniqueness tracking. Default: \"\""`
	Personal     bool                               `json:"personal" bson:"personal" jsonschema:"required,description=Whether this is a personal workspace. Default: false"`
	Policy       string                             `json:"policy" bson:"policy,omitempty" jsonschema:"description=Policy ID reference. Default: \"\""`
	SSOConfig    *WorkspaceSSOConfigDocument        `json:"ssoconfig,omitempty" bson:"ssoconfig,omitempty" jsonschema:"description=SSO configuration for enterprise workspaces"`
	UpdatedAt    time.Time                          `json:"updatedat" bson:"updatedat" jsonschema:"description=Last update timestamp"`
}

func NewWorkspace(ws *workspace.Workspace) (*WorkspaceDocument, string) {
	membersDoc := map[string]WorkspaceMemberDocument{}
	for uId, m := range ws.Members().Users() {
		membersDoc[uId.String()] = WorkspaceMemberDocument{
			Role:      string(m.Role),
			Disabled:  m.Disabled,
			InvitedBy: m.InvitedBy.String(),
		}
	}

	integrationsDoc := map[string]WorkspaceMemberDocument{}
	for iId, m := range ws.Members().Integrations() {
		integrationsDoc[iId.String()] = WorkspaceMemberDocument{
			Role:      string(m.Role),
			Disabled:  m.Disabled,
			InvitedBy: m.InvitedBy.String(),
		}
	}

	metadataDoc := WorkspaceMetadataDocument{
		Description:  ws.Metadata().Description(),
		Website:      ws.Metadata().Website(),
		Location:     ws.Metadata().Location(),
		BillingEmail: ws.Metadata().BillingEmail(),
		PhotoURL:     ws.Metadata().PhotoURL(),
	}

	// Compute members hash for unique indexing
	membersHash, err := ComputeWorkspaceMembersHash(membersDoc, integrationsDoc)
	if err != nil {
		// In case of marshalling error, fallback to empty hash
		// This should never happen with our data structures, but better to be safe
		membersHash = ""
	}

	var ssoDoc *WorkspaceSSOConfigDocument
	if sso := ws.SSOConfig(); sso != nil {
		ssoDoc = &WorkspaceSSOConfigDocument{
			Auth0ConnectionID: sso.Auth0ConnectionID(),
			Auth0OrgID:        sso.Auth0OrgID(),
			ClientID:          sso.ClientID(),
			ClientSecret:      sso.ClientSecret(),
			ConnectionType:    string(sso.ConnectionType()),
			DirectoryDomain:   sso.DirectoryDomain(),
			EmailDomains:      sso.EmailDomains(),
			Enabled:           sso.Enabled(),
			JITDefaultRole:    sso.JITDefaultRole(),
			OIDCClientID:      sso.OIDCClientID(),
			OIDCClientSecret:  sso.OIDCClientSecret(),
			OIDCDiscoveryURL:  sso.OIDCDiscoveryURL(),
			SAMLEntityID:      sso.SAMLEntityID(),
			SAMLMetadataURL:   sso.SAMLMetadataURL(),
			SAMLSignInURL:     sso.SAMLSignInURL(),
			SAMLSignOutURL:    sso.SAMLSignOutURL(),
			SAMLX509Cert:      sso.SAMLX509Cert(),
			Verified:          sso.Verified(),
		}
	}

	wId := ws.ID().String()
	updatedAt := ws.UpdatedAt()
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}
	return &WorkspaceDocument{
		ID:           wId,
		Name:         ws.Name(),
		Alias:        ws.Alias(),
		Email:        ws.Email(),
		Metadata:     metadataDoc,
		Members:      membersDoc,
		Integrations: integrationsDoc,
		MembersHash:  membersHash,
		Personal:     ws.IsPersonal(),
		Policy:       lo.FromPtr(ws.Policy()).String(),
		SSOConfig:    ssoDoc,
		UpdatedAt:    updatedAt,
	}, wId
}

func (d *WorkspaceDocument) Model() (*workspace.Workspace, error) {
	tid, err := id.WorkspaceIDFrom(d.ID)
	if err != nil {
		return nil, err
	}

	members := map[id.UserID]workspace.Member{}
	if d.Members != nil {
		for uid, member := range d.Members {
			uid, err := id.UserIDFrom(uid)
			if err != nil {
				return nil, err
			}
			inviterID, err := id.UserIDFrom(member.InvitedBy)
			if err != nil {
				inviterID = uid
			}
			members[uid] = workspace.Member{
				Role:      role.RoleType(member.Role),
				Disabled:  member.Disabled,
				InvitedBy: inviterID,
			}
		}
	}

	integrations := map[id.IntegrationID]workspace.Member{}
	if d.Integrations != nil {
		for iId, integrationDoc := range d.Integrations {
			iId, err := id.IntegrationIDFrom(iId)
			if err != nil {
				return nil, err
			}
			integrations[iId] = workspace.Member{
				Role:      role.RoleType(integrationDoc.Role),
				Disabled:  integrationDoc.Disabled,
				InvitedBy: id.MustUserID(integrationDoc.InvitedBy),
			}
		}
	}

	var policy *workspace.PolicyID
	if d.Policy != "" {
		policy = workspace.PolicyID(d.Policy).Ref()
	}

	metadata := workspace.MetadataFrom(d.Metadata.Description, d.Metadata.Website, d.Metadata.Location, d.Metadata.BillingEmail, d.Metadata.PhotoURL)

	var ssoConfig *sso.Config
	if d.SSOConfig != nil {
		ssoConfig = sso.New(sso.ConnectionType(d.SSOConfig.ConnectionType))
		ssoConfig.SetAuth0ConnectionID(d.SSOConfig.Auth0ConnectionID)
		ssoConfig.SetAuth0OrgID(d.SSOConfig.Auth0OrgID)
		ssoConfig.SetClientID(d.SSOConfig.ClientID)
		ssoConfig.SetClientSecret(d.SSOConfig.ClientSecret)
		ssoConfig.SetDirectoryDomain(d.SSOConfig.DirectoryDomain)
		ssoConfig.SetEmailDomains(d.SSOConfig.EmailDomains)
		ssoConfig.SetEnabled(d.SSOConfig.Enabled)
		ssoConfig.SetJITDefaultRole(d.SSOConfig.JITDefaultRole)
		ssoConfig.SetOIDCClientID(d.SSOConfig.OIDCClientID)
		ssoConfig.SetOIDCClientSecret(d.SSOConfig.OIDCClientSecret)
		ssoConfig.SetOIDCDiscoveryURL(d.SSOConfig.OIDCDiscoveryURL)
		ssoConfig.SetSAMLEntityID(d.SSOConfig.SAMLEntityID)
		ssoConfig.SetSAMLMetadataURL(d.SSOConfig.SAMLMetadataURL)
		ssoConfig.SetSAMLSignInURL(d.SSOConfig.SAMLSignInURL)
		ssoConfig.SetSAMLSignOutURL(d.SSOConfig.SAMLSignOutURL)
		ssoConfig.SetSAMLX509Cert(d.SSOConfig.SAMLX509Cert)
		ssoConfig.SetVerified(d.SSOConfig.Verified)
	}

	return workspace.New().
		ID(tid).
		Name(d.Name).
		Alias(d.Alias).
		Email(d.Email).
		Metadata(metadata).
		Members(members).
		Integrations(integrations).
		Personal(d.Personal).
		Policy(policy).
		SSOConfig(ssoConfig).
		UpdatedAt(d.UpdatedAt).
		Build()
}

func NewWorkspaces(workspaces []*workspace.Workspace) ([]*WorkspaceDocument, []string) {
	res := make([]*WorkspaceDocument, 0, len(workspaces))
	ids := make([]string, 0, len(workspaces))
	for _, d := range workspaces {
		if d == nil {
			continue
		}
		r, wId := NewWorkspace(d)
		res = append(res, r)
		ids = append(ids, wId)
	}
	return res, ids
}

type WorkspaceConsumer = Consumer[*WorkspaceDocument, *workspace.Workspace]

func NewWorkspaceConsumer() *WorkspaceConsumer {
	return NewConsumer[*WorkspaceDocument, *workspace.Workspace](func(a *workspace.Workspace) bool {
		return true
	})
}
