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

// compactingRepo returns found users reversed and without nil holes, as a
// repo that does not align its result might.
type compactingRepo struct{ user.Repo }

func (r compactingRepo) FindByIDs(ctx context.Context, ids user.IDList) (user.List, error) {
	found, err := r.Repo.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	var out user.List
	for i := len(found) - 1; i >= 0; i-- {
		if found[i] != nil {
			out = append(out, found[i])
		}
	}
	return out, nil
}

func TestMultiUser_FindByIDs_MatchesByIDForCompactingRepo(t *testing.T) {
	newUser := func(name string) *user.User {
		return user.New().NewID().Name(name).Email(name + "@example.com").Workspace(workspace.NewID()).MustBuild()
	}
	a, b := newUser("a"), newUser("b")
	missing := user.NewID()

	m := repo.NewMultiUser(compactingRepo{memory.NewUserWith(a, b)})
	got, err := m.FindByIDs(context.Background(), user.IDList{a.ID(), missing, b.ID()})
	require.NoError(t, err)
	assert.Equal(t, user.List{a, nil, b}, got)
}
