package rest

import (
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/pkg/sso"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/rerror"
)

type SSOHandler struct{}

func NewSSOHandler() *SSOHandler {
	return &SSOHandler{}
}

// ssoConfigResponse is returned by GET and PUT /workspaces/:id/sso.
// Secrets are masked in responses.
type ssoConfigResponse struct {
	Auth0ConnectionID string   `json:"auth0_connection_id,omitempty"`
	Auth0OrgID        string   `json:"auth0_org_id,omitempty"`
	ClientID          string   `json:"client_id,omitempty"`
	ClientSecret      string   `json:"client_secret,omitempty"`
	ConnectionType    string   `json:"connection_type"`
	DirectoryDomain   string   `json:"directory_domain,omitempty"`
	EmailDomains      []string `json:"email_domains"`
	Enabled           bool     `json:"enabled"`
	JITDefaultRole    string   `json:"jit_default_role,omitempty"`
	OIDCClientID      string   `json:"oidc_client_id,omitempty"`
	OIDCClientSecret  string   `json:"oidc_client_secret,omitempty"`
	OIDCDiscoveryURL  string   `json:"oidc_discovery_url,omitempty"`
	SAMLEntityID      string   `json:"saml_entity_id,omitempty"`
	SAMLMetadataURL   string   `json:"saml_metadata_url,omitempty"`
	SAMLSignInURL     string   `json:"saml_sign_in_url,omitempty"`
	SAMLSignOutURL    string   `json:"saml_sign_out_url,omitempty"`
	SAMLX509Cert      string   `json:"saml_x509_cert,omitempty"`
	Verified          bool     `json:"verified"`
}

type upsertSSORequest struct {
	ClientID         *string  `json:"client_id"`
	ClientSecret     *string  `json:"client_secret"`
	ConnectionType   string   `json:"connection_type"`
	DirectoryDomain  *string  `json:"directory_domain"`
	EmailDomains     []string `json:"email_domains"`
	Enabled          bool     `json:"enabled"`
	JITDefaultRole   string   `json:"jit_default_role"`
	OIDCClientID     *string  `json:"oidc_client_id"`
	OIDCClientSecret *string  `json:"oidc_client_secret"`
	OIDCDiscoveryURL *string  `json:"oidc_discovery_url"`
	SAMLEntityID     *string  `json:"saml_entity_id"`
	SAMLMetadataURL  *string  `json:"saml_metadata_url"`
	SAMLSignInURL    *string  `json:"saml_sign_in_url"`
	SAMLSignOutURL   *string  `json:"saml_sign_out_url"`
	SAMLX509Cert     *string  `json:"saml_x509_cert"`
}

type ssoLookupResponse struct {
	Auth0ConnectionName string `json:"auth0_connection_name"`
	Auth0OrgID          string `json:"auth0_org_id"`
	Required            bool   `json:"required"`
	WorkspaceID         string `json:"workspace_id"`
}

// DeleteSSOConfig handles DELETE /workspaces/:workspace_id/sso
func (h *SSOHandler) DeleteSSOConfig(c echo.Context) error {
	ctx := c.Request().Context()
	op := adapter.Operator(ctx)
	uc := adapter.Usecases(ctx)

	wsID, err := workspace.IDFrom(c.Param("workspace_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid workspace ID")
	}

	if err := uc.SSO.DeleteSSOConfig(ctx, wsID, op); err != nil {
		return toHTTPError(err)
	}

	return c.NoContent(http.StatusNoContent)
}

// GetSSOConfig handles GET /workspaces/:workspace_id/sso
func (h *SSOHandler) GetSSOConfig(c echo.Context) error {
	ctx := c.Request().Context()
	op := adapter.Operator(ctx)
	uc := adapter.Usecases(ctx)

	wsID, err := workspace.IDFrom(c.Param("workspace_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid workspace ID")
	}

	cfg, err := uc.SSO.GetSSOConfig(ctx, wsID, op)
	if err != nil {
		return toHTTPError(err)
	}

	return c.JSON(http.StatusOK, toSSOConfigResponse(cfg, true))
}

// Lookup handles GET /sso/lookup?email=<email>
func (h *SSOHandler) Lookup(c echo.Context) error {
	ctx := c.Request().Context()
	uc := adapter.Usecases(ctx)

	email := strings.TrimSpace(c.QueryParam("email"))
	if email == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "email query parameter is required")
	}

	result, err := uc.SSO.LookupByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, rerror.ErrNotFound) {
			return c.JSON(http.StatusOK, &ssoLookupResponse{Required: false})
		}
		return toHTTPError(err)
	}

	return c.JSON(http.StatusOK, &ssoLookupResponse{
		Auth0ConnectionName: result.Auth0ConnectionName,
		Auth0OrgID:          result.Auth0OrgID,
		Required:            result.Required,
		WorkspaceID:         result.WorkspaceID.String(),
	})
}

// UpsertSSOConfig handles PUT /workspaces/:workspace_id/sso
func (h *SSOHandler) UpsertSSOConfig(c echo.Context) error {
	ctx := c.Request().Context()
	op := adapter.Operator(ctx)
	uc := adapter.Usecases(ctx)

	wsID, err := workspace.IDFrom(c.Param("workspace_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid workspace ID")
	}

	var req upsertSSORequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	param := interfaces.UpsertSSOConfigParam{
		ClientID:         req.ClientID,
		ClientSecret:     req.ClientSecret,
		ConnectionType:   sso.ConnectionType(req.ConnectionType),
		DirectoryDomain:  req.DirectoryDomain,
		EmailDomains:     req.EmailDomains,
		Enabled:          req.Enabled,
		JITDefaultRole:   req.JITDefaultRole,
		OIDCClientID:     req.OIDCClientID,
		OIDCClientSecret: req.OIDCClientSecret,
		OIDCDiscoveryURL: req.OIDCDiscoveryURL,
		SAMLEntityID:     req.SAMLEntityID,
		SAMLMetadataURL:  req.SAMLMetadataURL,
		SAMLSignInURL:    req.SAMLSignInURL,
		SAMLSignOutURL:   req.SAMLSignOutURL,
		SAMLX509Cert:     req.SAMLX509Cert,
	}

	cfg, err := uc.SSO.UpsertSSOConfig(ctx, wsID, param, op)
	if err != nil {
		return toHTTPError(err)
	}

	return c.JSON(http.StatusOK, toSSOConfigResponse(cfg, true))
}

// VerifySSOConfig handles POST /workspaces/:workspace_id/sso/verify
func (h *SSOHandler) VerifySSOConfig(c echo.Context) error {
	ctx := c.Request().Context()
	op := adapter.Operator(ctx)
	uc := adapter.Usecases(ctx)

	wsID, err := workspace.IDFrom(c.Param("workspace_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid workspace ID")
	}

	if err := uc.SSO.VerifySSOConfig(ctx, wsID, op); err != nil {
		return toHTTPError(err)
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func toSSOConfigResponse(cfg *sso.Config, maskSecrets bool) *ssoConfigResponse {
	mask := func(s string) string {
		if maskSecrets && s != "" {
			return "***"
		}
		return s
	}

	domains := cfg.EmailDomains()
	if domains == nil {
		domains = []string{}
	}

	return &ssoConfigResponse{
		Auth0ConnectionID: cfg.Auth0ConnectionID(),
		Auth0OrgID:        cfg.Auth0OrgID(),
		ClientID:          cfg.ClientID(),
		ClientSecret:      mask(cfg.ClientSecret()),
		ConnectionType:    string(cfg.ConnectionType()),
		DirectoryDomain:   cfg.DirectoryDomain(),
		EmailDomains:      domains,
		Enabled:           cfg.Enabled(),
		JITDefaultRole:    cfg.JITDefaultRole(),
		OIDCClientID:      cfg.OIDCClientID(),
		OIDCClientSecret:  mask(cfg.OIDCClientSecret()),
		OIDCDiscoveryURL:  cfg.OIDCDiscoveryURL(),
		SAMLEntityID:      cfg.SAMLEntityID(),
		SAMLMetadataURL:   cfg.SAMLMetadataURL(),
		SAMLSignInURL:     cfg.SAMLSignInURL(),
		SAMLSignOutURL:    cfg.SAMLSignOutURL(),
		SAMLX509Cert:      cfg.SAMLX509Cert(),
		Verified:          cfg.Verified(),
	}
}

func toHTTPError(err error) error {
	if errors.Is(err, rerror.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	if errors.Is(err, interfaces.ErrSSONotEnterprise) {
		return echo.NewHTTPError(http.StatusForbidden, err.Error())
	}
	if errors.Is(err, interfaces.ErrSSOConfigNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}
	if errors.Is(err, interfaces.ErrOperationDenied) || errors.Is(err, interfaces.ErrPermissionDenied) {
		return echo.NewHTTPError(http.StatusForbidden, err.Error())
	}
	if ierr := rerror.UnwrapErrInternal(err); ierr != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}
	return echo.NewHTTPError(http.StatusBadRequest, err.Error())
}
