package gql

import (
	"context"
	"testing"

	"github.com/99designs/gqlgen/graphql"
	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/internal/adapter/gql/gqlmodel"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/idx"
	"github.com/stretchr/testify/assert"
	"github.com/vektah/gqlparser/v2/ast"
)

// userDataTypes carry user data, so every field must declare @visibility.
var userDataTypes = []string{"User", "Me", "UserMetadata", "Verification", "WorkspaceUserMember"}

func TestSchema_UserDataFieldsDeclareVisibility(t *testing.T) {
	s := NewExecutableSchema(Config{}).Schema()
	for _, name := range userDataTypes {
		def := s.Types[name]
		if !assert.NotNil(t, def, name) {
			continue
		}
		for _, f := range def.Fields {
			if f.Name == "__typename" {
				continue
			}
			assert.NotNil(t, f.Directives.ForName("visibility"), "%s.%s has no @visibility", name, f.Name)
		}
	}
}

type workspacesByID struct {
	interfaces.Workspace
	all workspace.List
}

func (u workspacesByID) Fetch(_ context.Context, ids workspace.IDList, _ *workspace.Operator) (workspace.List, error) {
	res := make(workspace.List, 0, len(ids))
	for _, i := range ids {
		for _, w := range u.all {
			if w.ID() == i {
				res = append(res, w)
			}
		}
	}
	return res, nil
}

type allUsersExist struct{ interfaces.User }

func (allUsersExist) FetchByID(_ context.Context, ids user.IDList) (user.List, error) {
	res := make(user.List, len(ids))
	for i, uid := range ids {
		res[i] = user.New().ID(uid).Name("u").Email("u@example.com").Workspace(id.NewWorkspaceID()).MustBuild()
	}
	return res, nil
}

func memberOf(users ...user.ID) *workspace.Workspace {
	m := map[idx.ID[id.User]]workspace.Member{}
	for _, u := range users {
		m[u] = workspace.Member{Role: role.RoleReader, InvitedBy: u}
	}
	return workspace.New().NewID().Name("w").Members(m).MustBuild()
}

func viewerCtx(viewer *user.User, all ...*workspace.Workspace) context.Context {
	uc := &interfaces.Container{Workspace: workspacesByID{all: all}, User: allUsersExist{}}
	ctx := AttachUsecases(adapter.AttachUsecases(context.Background(), uc), uc, adapter.PhotoSigner{}, true)
	if viewer == nil {
		return ctx
	}
	var mine id.WorkspaceIDList
	for _, w := range all {
		if w.Members().HasUser(viewer.ID()) {
			mine = append(mine, w.ID())
		}
	}
	ctx = adapter.AttachUser(ctx, viewer)
	vid := viewer.ID()
	return adapter.AttachOperator(ctx, &workspace.Operator{User: &vid, ReadableWorkspaces: mine})
}

func resolveEmail(ctx context.Context, owner *gqlmodel.User, level gqlmodel.VisibilityLevel) any {
	res, err := visibility(ctx, owner, func(context.Context) (any, error) { return owner.Email, nil }, level)
	if err != nil {
		panic(err)
	}
	return res
}

func TestVisibility(t *testing.T) {
	viewer := user.New().NewID().Name("viewer").Email("viewer@example.com").Workspace(id.NewWorkspaceID()).MustBuild()
	ownerID := user.NewID()
	stranger := user.NewID()
	owner := &gqlmodel.User{ID: gqlmodel.IDFrom(ownerID), Email: "owner@example.com"}
	shared := memberOf(viewer.ID(), ownerID)
	viewersOnly := memberOf(viewer.ID(), stranger)
	ownersOnly := memberOf(ownerID, stranger)

	tests := []struct {
		name  string
		ctx   context.Context
		level gqlmodel.VisibilityLevel
		want  any
	}{
		{"PUBLIC shows to anonymous", viewerCtx(nil), gqlmodel.VisibilityLevelPublic, "owner@example.com"},
		{"SELF hides from anonymous", viewerCtx(nil), gqlmodel.VisibilityLevelSelf, ""},
		{"SELF hides from a co-member", viewerCtx(viewer, shared), gqlmodel.VisibilityLevelSelf, ""},
		{"WORKSPACE hides from anonymous", viewerCtx(nil, shared), gqlmodel.VisibilityLevelWorkspace, ""},
		{"WORKSPACE shows to a co-member", viewerCtx(viewer, viewersOnly, shared), gqlmodel.VisibilityLevelWorkspace, "owner@example.com"},
		{"WORKSPACE hides when workspaces do not overlap", viewerCtx(viewer, viewersOnly, ownersOnly), gqlmodel.VisibilityLevelWorkspace, ""},
		{"WORKSPACE hides from a viewer with no workspace", viewerCtx(viewer, ownersOnly), gqlmodel.VisibilityLevelWorkspace, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, resolveEmail(tt.ctx, owner, tt.level))
		})
	}

	t.Run("SELF and WORKSPACE show to the owner", func(t *testing.T) {
		self := &gqlmodel.User{ID: gqlmodel.IDFrom(viewer.ID()), Email: "viewer@example.com"}
		assert.Equal(t, "viewer@example.com", resolveEmail(viewerCtx(viewer), self, gqlmodel.VisibilityLevelSelf))
		assert.Equal(t, "viewer@example.com", resolveEmail(viewerCtx(viewer), self, gqlmodel.VisibilityLevelWorkspace))
	})

	t.Run("a field below the owner finds it through the parent field", func(t *testing.T) {
		ctx := graphql.WithFieldContext(viewerCtx(nil), &graphql.FieldContext{Result: &owner})
		meta := &gqlmodel.UserMetadata{Lang: "ja"}
		res, err := visibility(ctx, meta, func(context.Context) (any, error) { return meta.Lang, nil }, gqlmodel.VisibilityLevelSelf)
		assert.NoError(t, err)
		assert.Zero(t, res)
	})

	t.Run("a non-null object is hidden as an empty object", func(t *testing.T) {
		ctx := graphql.WithFieldContext(viewerCtx(nil), &graphql.FieldContext{Field: graphql.CollectedField{Field: &ast.Field{Definition: &ast.FieldDefinition{Type: ast.NonNullNamedType("UserMetadata", nil)}}}})
		meta := &gqlmodel.UserMetadata{Description: "secret"}
		res, err := visibility(ctx, owner, func(context.Context) (any, error) { return meta, nil }, gqlmodel.VisibilityLevelSelf)
		assert.NoError(t, err)
		assert.Equal(t, &gqlmodel.UserMetadata{}, res)
	})
}
