package interfaces

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/sso"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/rerror"
)

var (
	ErrSSOConfigNotFound = rerror.NewE(i18n.T("SSO configuration not found"))
	ErrSSONotEnterprise  = rerror.NewE(i18n.T("SSO requires enterprise plan"))
)

type UpsertSSOConfigParam struct {
	ClientID         *string
	ClientSecret     *string
	ConnectionType   sso.ConnectionType
	DirectoryDomain  *string
	EmailDomains     []string
	Enabled          bool
	JITDefaultRole   string
	OIDCClientID     *string
	OIDCClientSecret *string
	OIDCDiscoveryURL *string
	SAMLEntityID     *string
	SAMLMetadataURL  *string
	SAMLSignInURL    *string
	SAMLSignOutURL   *string
	SAMLX509Cert     *string
}

type SSOLookupResult struct {
	Auth0ConnectionName string
	Auth0OrgID          string
	Required            bool
	WorkspaceID         workspace.ID
}

type SSO interface {
	DeleteSSOConfig(ctx context.Context, workspaceID workspace.ID, operator *workspace.Operator) error
	GetSSOConfig(ctx context.Context, workspaceID workspace.ID, operator *workspace.Operator) (*sso.Config, error)
	LookupByEmail(ctx context.Context, email string) (*SSOLookupResult, error)
	UpsertSSOConfig(ctx context.Context, workspaceID workspace.ID, param UpsertSSOConfigParam, operator *workspace.Operator) (*sso.Config, error)
	VerifySSOConfig(ctx context.Context, workspaceID workspace.ID, operator *workspace.Operator) error
}
