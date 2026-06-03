package interactor

import (
	"context"
	"errors"
	"strings"

	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/sso"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/rerror"
)

type ssoInteractor struct {
	repos *repo.Container
}

func NewSSO(r *repo.Container) interfaces.SSO {
	return &ssoInteractor{repos: r}
}

func (s *ssoInteractor) DeleteSSOConfig(ctx context.Context, workspaceID workspace.ID, operator *workspace.Operator) error {
	return Run0(ctx, operator, s.repos,
		Usecase().WithOwnableWorkspaces(workspaceID).Transaction(),
		func(ctx context.Context) error {
			ws, err := s.repos.Workspace.FindByID(ctx, workspaceID)
			if err != nil {
				return err
			}

			if !ws.IsEnterprise() {
				return interfaces.ErrSSONotEnterprise
			}

			if ws.SSOConfig() == nil {
				return interfaces.ErrSSOConfigNotFound
			}

			ws.DeleteSSOConfig()
			return s.repos.Workspace.Save(ctx, ws)
		})
}

func (s *ssoInteractor) GetSSOConfig(ctx context.Context, workspaceID workspace.ID, operator *workspace.Operator) (*sso.Config, error) {
	return Run1(ctx, operator, s.repos,
		Usecase().WithReadableWorkspaces(workspaceID),
		func(ctx context.Context) (*sso.Config, error) {
			ws, err := s.repos.Workspace.FindByID(ctx, workspaceID)
			if err != nil {
				return nil, err
			}

			if !ws.IsEnterprise() {
				return nil, interfaces.ErrSSONotEnterprise
			}

			cfg := ws.SSOConfig()
			if cfg == nil {
				return nil, interfaces.ErrSSOConfigNotFound
			}

			return cfg, nil
		})
}

func (s *ssoInteractor) LookupByEmail(ctx context.Context, email string) (*interfaces.SSOLookupResult, error) {
	domain := extractDomain(email)
	if domain == "" {
		return nil, rerror.ErrNotFound
	}

	ws, err := s.repos.Workspace.FindByEmailDomain(ctx, domain)
	if err != nil {
		if errors.Is(err, rerror.ErrNotFound) {
			return nil, rerror.ErrNotFound
		}
		return nil, err
	}

	cfg := ws.SSOConfig()
	if cfg == nil || !cfg.Enabled() {
		return nil, rerror.ErrNotFound
	}

	return &interfaces.SSOLookupResult{
		Auth0ConnectionName: cfg.Auth0ConnectionID(),
		Auth0OrgID:          cfg.Auth0OrgID(),
		Required:            true,
		WorkspaceID:         ws.ID(),
	}, nil
}

func (s *ssoInteractor) UpsertSSOConfig(ctx context.Context, workspaceID workspace.ID, param interfaces.UpsertSSOConfigParam, operator *workspace.Operator) (*sso.Config, error) {
	return Run1(ctx, operator, s.repos,
		Usecase().WithOwnableWorkspaces(workspaceID).Transaction(),
		func(ctx context.Context) (*sso.Config, error) {
			ws, err := s.repos.Workspace.FindByID(ctx, workspaceID)
			if err != nil {
				return nil, err
			}

			if !ws.IsEnterprise() {
				return nil, interfaces.ErrSSONotEnterprise
			}

			cfg := ws.SSOConfig()
			if cfg == nil {
				cfg = sso.New(param.ConnectionType)
			}

			cfg.SetConnectionType(param.ConnectionType)
			cfg.SetEmailDomains(param.EmailDomains)
			cfg.SetEnabled(param.Enabled)
			cfg.SetJITDefaultRole(param.JITDefaultRole)

			if param.ClientID != nil {
				cfg.SetClientID(*param.ClientID)
			}
			if param.ClientSecret != nil {
				cfg.SetClientSecret(*param.ClientSecret)
			}
			if param.DirectoryDomain != nil {
				cfg.SetDirectoryDomain(*param.DirectoryDomain)
			}

			if param.OIDCClientID != nil {
				cfg.SetOIDCClientID(*param.OIDCClientID)
			}
			if param.OIDCClientSecret != nil {
				cfg.SetOIDCClientSecret(*param.OIDCClientSecret)
			}
			if param.OIDCDiscoveryURL != nil {
				cfg.SetOIDCDiscoveryURL(*param.OIDCDiscoveryURL)
			}

			if param.SAMLEntityID != nil {
				cfg.SetSAMLEntityID(*param.SAMLEntityID)
			}
			if param.SAMLMetadataURL != nil {
				cfg.SetSAMLMetadataURL(*param.SAMLMetadataURL)
			}
			if param.SAMLSignInURL != nil {
				cfg.SetSAMLSignInURL(*param.SAMLSignInURL)
			}
			if param.SAMLSignOutURL != nil {
				cfg.SetSAMLSignOutURL(*param.SAMLSignOutURL)
			}
			if param.SAMLX509Cert != nil {
				cfg.SetSAMLX509Cert(*param.SAMLX509Cert)
			}

			cfg.SetVerified(false)

			ws.SetSSOConfig(cfg)
			if err := s.repos.Workspace.Save(ctx, ws); err != nil {
				return nil, err
			}

			return cfg, nil
		})
}

func (s *ssoInteractor) VerifySSOConfig(ctx context.Context, workspaceID workspace.ID, operator *workspace.Operator) error {
	return Run0(ctx, operator, s.repos,
		Usecase().WithOwnableWorkspaces(workspaceID).Transaction(),
		func(ctx context.Context) error {
			ws, err := s.repos.Workspace.FindByID(ctx, workspaceID)
			if err != nil {
				return err
			}

			if !ws.IsEnterprise() {
				return interfaces.ErrSSONotEnterprise
			}

			cfg := ws.SSOConfig()
			if cfg == nil {
				return interfaces.ErrSSOConfigNotFound
			}

			// TODO(U-D20-03): call Auth0 Management API to validate connection
			cfg.SetVerified(true)
			ws.SetSSOConfig(cfg)
			return s.repos.Workspace.Save(ctx, ws)
		})
}

func extractDomain(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return ""
	}
	return strings.ToLower(parts[1])
}
