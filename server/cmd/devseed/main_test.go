package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/joho/godotenv"
	"github.com/reearth/reearth-accounts/server/internal/app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testOptions(dir string) options {
	return options{
		dir:      dir,
		host:     "mock-oauth2",
		port:     8091,
		issuerID: "accounts-local",
		audience: "reearth-accounts-local",
		clients:  []client{{id: "cms", actAs: []string{"integration"}}, {id: "anon", actAs: []string{}}},
	}
}

func seedInto(t *testing.T, o options) string {
	t.Helper()
	var out bytes.Buffer
	require.NoError(t, seed(o, &out))
	return out.String()
}

func mode(t *testing.T, p string) os.FileMode {
	t.Helper()
	fi, err := os.Stat(p)
	require.NoError(t, err)
	return fi.Mode().Perm()
}

func TestSeed_WritesKeysAndEnvWithPrivateModes(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "service-callers")
	o := testOptions(dir)
	out := seedInto(t, o)

	assert.Equal(t, os.FileMode(0o700), mode(t, dir))
	for _, c := range o.clients {
		fi, err := os.Stat(o.keyPath(c))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), fi.Mode().Perm(), c.id)
		assert.Greater(t, fi.Size(), int64(1000), c.id)
	}
	assert.Equal(t, os.FileMode(0o600), mode(t, filepath.Join(dir, envFileName)))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	assert.ElementsMatch(t, []string{"cms.key", "anon.key", envFileName}, names)

	assert.NotContains(t, out, "PRIVATE KEY")
	assert.NotContains(t, out, "-----BEGIN")
	assert.Contains(t, out, "token URL: http://mock-oauth2:8091/accounts-local/token")
	assert.Contains(t, out, "client ID: cms")
	assert.Contains(t, out, "audience:  reearth-accounts-local")
	assert.Contains(t, out, "key file:  "+filepath.Join(dir, "cms.key"))
	assert.Contains(t, out, "127.0.0.1 mock-oauth2")
}

func TestSeed_KeepsKeysUnlessRotate(t *testing.T) {
	dir := t.TempDir()
	o := testOptions(dir)
	seedInto(t, o)
	first, err := os.Stat(o.keyPath(o.clients[0]))
	require.NoError(t, err)
	env1, err := os.ReadFile(filepath.Join(dir, envFileName))
	require.NoError(t, err)

	seedInto(t, o)
	second, err := os.Stat(o.keyPath(o.clients[0]))
	require.NoError(t, err)
	assert.True(t, os.SameFile(first, second), "second run keeps the key")
	assert.Equal(t, first.ModTime(), second.ModTime())
	env2, err := os.ReadFile(filepath.Join(dir, envFileName))
	require.NoError(t, err)
	assert.Equal(t, env1, env2)

	o.rotate = true
	seedInto(t, o)
	rotated, err := os.Stat(o.keyPath(o.clients[0]))
	require.NoError(t, err)
	assert.False(t, os.SameFile(first, rotated), "-rotate replaces the key")
	assert.Equal(t, os.FileMode(0o600), rotated.Mode().Perm())
}

func TestSeed_RefusesSymlinkKey(t *testing.T) {
	dir := t.TempDir()
	o := testOptions(dir)
	require.NoError(t, os.Symlink(filepath.Join(t.TempDir(), "elsewhere"), o.keyPath(o.clients[0])))
	assert.ErrorContains(t, seed(o, &bytes.Buffer{}), "not a regular file")
}

func loadEnv(t *testing.T, path string) map[string]string {
	t.Helper()
	env, err := godotenv.Read(path)
	require.NoError(t, err)
	return env
}

func TestSeed_EnvParsesWithAccountsDecoders(t *testing.T) {
	dir := t.TempDir()
	seedInto(t, testOptions(dir))
	env := loadEnv(t, filepath.Join(dir, envFileName))

	assert.Equal(t, "true", env["REEARTH_ACCOUNTS_LOCAL_ISSUER"])

	var auth app.AuthConfigs
	require.NoError(t, auth.Decode(env["REEARTH_AUTH"]))
	require.Len(t, auth, 1)
	assert.Equal(t, "http://mock-oauth2:8091/accounts-local", auth[0].ISS)
	assert.Equal(t, []string{"reearth-accounts-local"}, auth[0].AUD)

	var callers app.ServiceCallers
	require.NoError(t, callers.Decode(env["REEARTH_ACCOUNTS_SERVICE_CALLERS"]))
	assert.Len(t, callers, 2)
}

func TestSeed_EnvPassesReadConfigOnlyWithLocalIssuerSwitch(t *testing.T) {
	dir := t.TempDir()
	seedInto(t, testOptions(dir))
	env := loadEnv(t, filepath.Join(dir, envFileName))

	t.Setenv("SKIP_DOTENV", "1")
	t.Setenv("K_SERVICE", "")
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	for k, v := range env {
		t.Setenv(k, v)
	}
	cfg, err := app.ReadConfig(false)
	require.NoError(t, err)
	assert.True(t, cfg.LocalIssuer)
	assert.Len(t, cfg.ServiceCallers, 2)

	t.Setenv("REEARTH_ACCOUNTS_LOCAL_ISSUER", "")
	_, err = app.ReadConfig(false)
	assert.ErrorContains(t, err, "untrusted token URL")
}

func TestGuardDir(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	got, err := guardDir(root, ".dev/service-callers")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, ".dev", "service-callers"), got)

	for _, dir := range []string{".", ".dev/..", "../x", outside, filepath.Join(root, ".devx")} {
		_, err := guardDir(root, dir)
		assert.Error(t, err, dir)
	}

	require.NoError(t, os.Symlink(outside, filepath.Join(root, ".dev")))
	_, err = guardDir(root, ".dev/service-callers")
	assert.ErrorContains(t, err, "resolves outside")
}

func TestRun_RefusesDirOutsideIgnoredDirectory(t *testing.T) {
	err := run([]string{"-dir", t.TempDir()}, &bytes.Buffer{})
	assert.ErrorContains(t, err, "outside")
}

func TestClientFlag(t *testing.T) {
	var f clientFlags
	require.NoError(t, f.Set("cms=integration"))
	require.NoError(t, f.Set("anon="))
	assert.Equal(t, clientFlags{{id: "cms", actAs: []string{"integration"}}, {id: "anon", actAs: []string{}}}, f)
	for _, bad := range []string{"cms", "../x=integration", "CMS=integration", "=integration"} {
		assert.Error(t, f.Set(bad), bad)
	}
}
