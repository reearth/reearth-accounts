package e2e

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/reearth/reearth-accounts/server/e2e/compat/oldclient/gqlclient"
	"github.com/reearth/reearth-accounts/server/internal/app"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/stretchr/testify/require"
)

// Clients before #374 (pseudo-version below) reject the empty email the
// server returns for other users. A service on such a client may call only
// FindMe and the workspace reads.
const clientAcceptsHiddenEmailFrom = "v0.0.0-20261106140637"

// ADR-0003 decision 8: the oldest pkg/gqlclient a service still builds with
// must keep working against the current server. e2e/compat/oldclient/VERSION
// names it; refresh it with e2e/compat/sync.sh when services upgrade.
func TestOldestGQLClient(t *testing.T) {
	raw, err := os.ReadFile("compat/oldclient/VERSION")
	require.NoError(t, err)
	version := strings.TrimSpace(string(raw))
	t.Logf("client version %s", version)

	ctx := context.Background()
	repos := memory.New()
	require.NoError(t, seedEmailVisibility(ctx, repos))
	url := startServerURL(t, &app.Config{}, repos, nil, true)
	c := gqlclient.NewClient(url, 10, gqlclient.NewAccountsTransport(debugUserTransport{uId2.String()}, "compat-test"))

	t.Run("own record and workspaces", func(t *testing.T) {
		me, err := c.UserRepo.FindMe(ctx)
		require.NoError(t, err)
		require.Equal(t, viewerEmail, me.Email())

		_, err = c.WorkspaceRepo.FindByID(ctx, wId.String())
		require.NoError(t, err)
		_, err = c.WorkspaceRepo.FindByAlias(ctx, "team-alias")
		require.NoError(t, err)
		ws, err := c.WorkspaceRepo.FindByUser(ctx, uId2.String())
		require.NoError(t, err)
		require.NotEmpty(t, ws)
	})

	t.Run("other users", func(t *testing.T) {
		lookups := map[string]func() error{
			"FindByID": func() error { _, err := c.UserRepo.FindByID(ctx, uId.String()); return err },
			"FindByIDs": func() error {
				_, err := c.UserRepo.FindByIDs(ctx, []string{uId.String(), uId2.String()})
				return err
			},
			"FindByAlias": func() error { _, err := c.UserRepo.FindByAlias(ctx, "victim"); return err },
			"FindUsersByIDsWithPagination": func() error {
				_, _, err := c.UserRepo.FindUsersByIDsWithPagination(ctx, []string{uId.String()}, "", 1, 10)
				return err
			},
		}
		for name, lookup := range lookups {
			t.Run(name, func(t *testing.T) {
				err := recovered(lookup)
				if version < clientAcceptsHiddenEmailFrom {
					require.ErrorContains(t, err, "invalid email", "server must not return other users' email")
					return
				}
				require.NoError(t, err)
			})
		}
	})
}

// recovered turns a panic into an error: some old client calls panic on data
// they reject instead of returning an error.
func recovered(f func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return f()
}
