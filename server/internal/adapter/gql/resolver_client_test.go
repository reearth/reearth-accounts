package gql

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/internal/adapter/gql/gqlmodel"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
)

func TestUserDirectoryRefusesAClientActingForAUser(t *testing.T) {
	u := user.New().NewID().Name("u").Email("u@example.com").MustBuild()
	uid := u.ID()
	uc := &interfaces.Container{}
	ctx := adapter.AttachUser(AttachUsecases(context.Background(), uc, adapter.PhotoSigner{}, false), u)
	ctx = adapter.AttachOperator(ctx, &workspace.Operator{User: &uid, Client: &workspace.Client{ID: "svc", Name: "svc"}})
	r := &queryResolver{&Resolver{}}
	id := gqlmodel.IDFrom(uid)

	calls := map[string]func() error{
		"node(USER)":        func() error { _, err := r.Node(ctx, id, gqlmodel.NodeTypeUser); return err },
		"nodes(USER)":       func() error { _, err := r.Nodes(ctx, []gqlmodel.ID{id}, gqlmodel.NodeTypeUser); return err },
		"user":              func() error { _, err := r.User(ctx, id); return err },
		"userByNameOrEmail": func() error { _, err := r.UserByNameOrEmail(ctx, "u"); return err },
		"userByNameOrAlias": func() error { _, err := r.UserByNameOrAlias(ctx, "u"); return err },
		"searchUser":        func() error { _, err := r.SearchUser(ctx, "u"); return err },
		"findUserByAlias":   func() error { _, err := r.FindUserByAlias(ctx, "u"); return err },
		"findUsersByIDs":    func() error { _, err := r.FindUsersByIDs(ctx, []gqlmodel.ID{id}); return err },
		"findUsersByIDsWithPagination": func() error {
			_, err := r.FindUsersByIDsWithPagination(ctx, []gqlmodel.ID{id}, nil, gqlmodel.Pagination{Page: 1, Size: 1})
			return err
		},
	}
	for name, call := range calls {
		assert.ErrorIs(t, call(), interfaces.ErrInvalidOperator, name)
	}
}
