package repo_test

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMultiUser_FindByIDs_AlignsWithIDs(t *testing.T) {
	newUser := func(uid user.ID, name string) *user.User {
		return user.New().ID(uid).Name(name).Email(name + "@example.com").Workspace(workspace.NewID()).MustBuild()
	}
	a := newUser(user.NewID(), "a")
	bID := user.NewID()
	bFirst, bSecond := newUser(bID, "b-first"), newUser(bID, "b-second")
	c := newUser(user.NewID(), "c")
	missing := user.NewID()

	m := repo.NewMultiUser(memory.NewUserWith(a, bFirst), memory.NewUserWith(bSecond, c))
	got, err := m.FindByIDs(context.Background(), user.IDList{c.ID(), missing, bID, a.ID()})
	require.NoError(t, err)
	assert.Equal(t, user.List{c, nil, bFirst, a}, got)
}
