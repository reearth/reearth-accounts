package interactor

import (
	"context"
	"errors"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/sso"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/rerror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSSOTest(t *testing.T) (*ssoInteractor, *workspace.Workspace, *workspace.Operator) {
	t.Helper()

	enterprisePolicy := workspace.PolicyEnterprise
	ws := workspace.New().NewID().Name("Enterprise Corp").Personal(false).Policy(&enterprisePolicy).MustBuild()

	wsRepo := memory.NewWorkspaceWith(ws)
	r := &repo.Container{Workspace: wsRepo}

	uc := &ssoInteractor{repos: r}
	op := &workspace.Operator{
		OwningWorkspaces: workspace.IDList{ws.ID()},
	}

	return uc, ws, op
}

func TestSSO_UpsertSSOConfig_Enterprise(t *testing.T) {
	uc, ws, op := setupSSOTest(t)
	ctx := context.Background()

	param := interfaces.UpsertSSOConfigParam{
		ConnectionType: sso.ConnectionTypeSAML,
		Enabled:        true,
		EmailDomains:   []string{"corp.com"},
		JITDefaultRole: "member",
		SAMLSignInURL:  strPtr("https://idp.corp.com/sso"),
		SAMLX509Cert:   strPtr("-----BEGIN CERTIFICATE-----\nfake"),
	}

	cfg, err := uc.UpsertSSOConfig(ctx, ws.ID(), param, op)
	require.NoError(t, err)
	assert.Equal(t, sso.ConnectionTypeSAML, cfg.ConnectionType())
	assert.True(t, cfg.Enabled())
	assert.Equal(t, []string{"corp.com"}, cfg.EmailDomains())
	assert.Equal(t, "member", cfg.JITDefaultRole())
	assert.Equal(t, "https://idp.corp.com/sso", cfg.SAMLSignInURL())
	assert.False(t, cfg.Verified(), "upsert must reset verified to false")
}

func TestSSO_UpsertSSOConfig_NonEnterprise(t *testing.T) {
	freeWS := workspace.New().NewID().Name("Free Workspace").Personal(false).MustBuild()
	wsRepo := memory.NewWorkspaceWith(freeWS)
	r := &repo.Container{Workspace: wsRepo}
	uc := &ssoInteractor{repos: r}

	op := &workspace.Operator{OwningWorkspaces: workspace.IDList{freeWS.ID()}}
	ctx := context.Background()

	param := interfaces.UpsertSSOConfigParam{
		ConnectionType: sso.ConnectionTypeOIDC,
	}

	_, err := uc.UpsertSSOConfig(ctx, freeWS.ID(), param, op)
	assert.ErrorIs(t, err, interfaces.ErrSSONotEnterprise)
}

func TestSSO_GetSSOConfig_Found(t *testing.T) {
	uc, ws, _ := setupSSOTest(t)
	ctx := context.Background()

	cfg := sso.New(sso.ConnectionTypeOIDC)
	cfg.SetEnabled(true)
	cfg.SetEmailDomains([]string{"example.com"})
	ws.SetSSOConfig(cfg)
	require.NoError(t, uc.repos.Workspace.Save(ctx, ws))

	readOp := &workspace.Operator{ReadableWorkspaces: workspace.IDList{ws.ID()}}

	got, err := uc.GetSSOConfig(ctx, ws.ID(), readOp)
	require.NoError(t, err)
	assert.Equal(t, sso.ConnectionTypeOIDC, got.ConnectionType())
	assert.True(t, got.Enabled())
}

func TestSSO_GetSSOConfig_NotFound(t *testing.T) {
	uc, ws, _ := setupSSOTest(t)
	ctx := context.Background()

	readOp := &workspace.Operator{ReadableWorkspaces: workspace.IDList{ws.ID()}}

	_, err := uc.GetSSOConfig(ctx, ws.ID(), readOp)
	assert.ErrorIs(t, err, interfaces.ErrSSOConfigNotFound)
}

func TestSSO_DeleteSSOConfig(t *testing.T) {
	uc, ws, op := setupSSOTest(t)
	ctx := context.Background()

	cfg := sso.New(sso.ConnectionTypeSAML)
	cfg.SetEnabled(true)
	ws.SetSSOConfig(cfg)
	require.NoError(t, uc.repos.Workspace.Save(ctx, ws))

	err := uc.DeleteSSOConfig(ctx, ws.ID(), op)
	require.NoError(t, err)

	updated, err := uc.repos.Workspace.FindByID(ctx, ws.ID())
	require.NoError(t, err)
	assert.Nil(t, updated.SSOConfig())
}

func TestSSO_DeleteSSOConfig_ConfigNotFound(t *testing.T) {
	uc, ws, op := setupSSOTest(t)
	ctx := context.Background()

	err := uc.DeleteSSOConfig(ctx, ws.ID(), op)
	assert.ErrorIs(t, err, interfaces.ErrSSOConfigNotFound)
}

func TestSSO_LookupByEmail(t *testing.T) {
	enterprisePolicy := workspace.PolicyEnterprise
	ws := workspace.New().NewID().Name("Corp").Personal(false).Policy(&enterprisePolicy).MustBuild()
	cfg := sso.New(sso.ConnectionTypeAzureAD)
	cfg.SetEnabled(true)
	cfg.SetEmailDomains([]string{"corp.com"})
	cfg.SetAuth0ConnectionID("con_abc")
	cfg.SetAuth0OrgID("org_xyz")
	ws.SetSSOConfig(cfg)

	wsRepo := memory.NewWorkspaceWith(ws)
	r := &repo.Container{Workspace: wsRepo}
	uc := &ssoInteractor{repos: r}
	ctx := context.Background()

	result, err := uc.LookupByEmail(ctx, "user@corp.com")
	require.NoError(t, err)
	assert.True(t, result.Required)
	assert.Equal(t, ws.ID(), result.WorkspaceID)
	assert.Equal(t, "con_abc", result.Auth0ConnectionName)
	assert.Equal(t, "org_xyz", result.Auth0OrgID)
}

func TestSSO_LookupByEmail_NotFound(t *testing.T) {
	wsRepo := memory.NewWorkspace()
	r := &repo.Container{Workspace: wsRepo}
	uc := &ssoInteractor{repos: r}
	ctx := context.Background()

	_, err := uc.LookupByEmail(ctx, "user@unknown.com")
	assert.True(t, errors.Is(err, rerror.ErrNotFound))
}

func TestSSO_LookupByEmail_InvalidEmail(t *testing.T) {
	wsRepo := memory.NewWorkspace()
	r := &repo.Container{Workspace: wsRepo}
	uc := &ssoInteractor{repos: r}
	ctx := context.Background()

	_, err := uc.LookupByEmail(ctx, "notanemail")
	assert.True(t, errors.Is(err, rerror.ErrNotFound))
}

func TestSSO_VerifySSOConfig_Placeholder(t *testing.T) {
	uc, ws, op := setupSSOTest(t)
	ctx := context.Background()

	cfg := sso.New(sso.ConnectionTypeSAML)
	cfg.SetEnabled(true)
	ws.SetSSOConfig(cfg)
	require.NoError(t, uc.repos.Workspace.Save(ctx, ws))

	err := uc.VerifySSOConfig(ctx, ws.ID(), op)
	require.NoError(t, err)

	updated, err := uc.repos.Workspace.FindByID(ctx, ws.ID())
	require.NoError(t, err)
	assert.True(t, updated.SSOConfig().Verified())
}

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		email  string
		domain string
	}{
		{"user@example.com", "example.com"},
		{"USER@EXAMPLE.COM", "example.com"},
		{"user@sub.domain.org", "sub.domain.org"},
		{"notanemail", ""},
		{"@nodomain", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			assert.Equal(t, tt.domain, extractDomain(tt.email))
		})
	}
}
