package interactor

import (
	"context"
	"errors"
	"testing"

	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type compactingUserRepo struct {
	user.Repo
	users user.List
}

func (r compactingUserRepo) FindByIDs(_ context.Context, ids user.IDList) (user.List, error) {
	var res user.List
	for i := len(r.users) - 1; i >= 0; i-- {
		if ids.Has(r.users[i].ID()) {
			res = append(res, r.users[i])
		}
	}
	return res, nil
}

func TestUserQuery_FetchByID_AlignsWithIDs(t *testing.T) {
	newUser := func(name string) *user.User {
		return user.New().NewID().Name(name).Email(name + "@example.com").Workspace(workspace.NewID()).MustBuild()
	}
	a, b, c := newUser("a"), newUser("b"), newUser("c")
	missing := user.NewID()

	q := NewUserQuery(compactingUserRepo{users: user.List{a, b}}, compactingUserRepo{users: user.List{c}})
	got, err := q.FetchByID(context.Background(), user.IDList{a.ID(), missing, c.ID(), b.ID()})
	require.NoError(t, err)
	assert.Equal(t, user.List{a, nil, c, b}, got)
}

type failingUserRepo struct {
	user.Repo
}

func (failingUserRepo) FindByIDs(context.Context, user.IDList) (user.List, error) {
	return nil, errors.New("unreachable repo")
}

func TestUserQuery_FetchByID_StopsOnceAllFound(t *testing.T) {
	a := user.New().NewID().Name("a").Email("a@example.com").Workspace(workspace.NewID()).MustBuild()

	q := NewUserQuery(compactingUserRepo{users: user.List{a}}, failingUserRepo{})
	got, err := q.FetchByID(context.Background(), user.IDList{a.ID()})
	require.NoError(t, err)
	assert.Equal(t, user.List{a}, got)

	_, err = q.FetchByID(context.Background(), user.IDList{a.ID(), user.NewID()})
	assert.EqualError(t, err, "unreachable repo")
}
