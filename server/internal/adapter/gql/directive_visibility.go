package gql

import (
	"context"
	"reflect"

	"github.com/99designs/gqlgen/graphql"
	"github.com/reearth/reearth-accounts/server/internal/adapter/gql/gqlmodel"
)

func NewDirectives() DirectiveRoot {
	return DirectiveRoot{Visibility: visibility}
}

// visibility enforces @visibility (ADR-0003 decision 4). The owner is the
// nearest object, from the field's parent upwards, that names a user. A field
// with no owner in reach is hidden unless PUBLIC.
func visibility(ctx context.Context, obj any, next graphql.Resolver, level gqlmodel.VisibilityLevel) (any, error) {
	res, err := next(ctx)
	if err != nil || level == gqlmodel.VisibilityLevelPublic {
		return res, err
	}
	if owner, ok := ownerOf(ctx, obj); ok && canSee(ctx, level, owner) {
		return res, nil
	}
	return hiddenValue(ctx, res), nil
}

func ownerOf(ctx context.Context, obj any) (gqlmodel.VisibilityOwner, bool) {
	if o, ok := asOwner(obj); ok {
		return o, true
	}
	for fc := graphql.GetFieldContext(ctx); fc != nil; fc = fc.Parent {
		if o, ok := asOwner(fc.Result); ok {
			return o, true
		}
	}
	return gqlmodel.VisibilityOwner{}, false
}

type visibilityOwned interface {
	VisibilityOwner() gqlmodel.VisibilityOwner
}

// asOwner unwraps pointers because gqlgen stores list items as **T.
func asOwner(v any) (gqlmodel.VisibilityOwner, bool) {
	for v != nil {
		if o, ok := v.(visibilityOwned); ok {
			rv := reflect.ValueOf(v)
			if rv.Kind() == reflect.Pointer && rv.IsNil() {
				return gqlmodel.VisibilityOwner{}, false
			}
			return o.VisibilityOwner(), true
		}
		rv := reflect.ValueOf(v)
		if rv.Kind() != reflect.Pointer || rv.IsNil() {
			return gqlmodel.VisibilityOwner{}, false
		}
		v = rv.Elem().Interface()
	}
	return gqlmodel.VisibilityOwner{}, false
}

func canSee(ctx context.Context, level gqlmodel.VisibilityLevel, owner gqlmodel.VisibilityOwner) bool {
	if owner.OwnRecord {
		return true
	}
	viewer := getUser(ctx)
	if viewer == nil {
		return false
	}
	if gqlmodel.ID(viewer.ID().String()) == owner.UserID {
		return true
	}
	if level == gqlmodel.VisibilityLevelWorkspace {
		return sharesWorkspace(ctx, owner.UserID)
	}
	return false
}

// sharesWorkspace reports whether owner is a user member, with any role, of a
// workspace the viewer is a user member of. Personal workspaces count.
// Integration members are not users and never match.
func sharesWorkspace(ctx context.Context, owner gqlmodel.ID) bool {
	wids := getOperator(ctx).AllReadableWorkspaces()
	if len(wids) == 0 {
		return false
	}
	ids := make([]gqlmodel.ID, len(wids))
	for i, w := range wids {
		ids[i] = gqlmodel.IDFrom(w)
	}
	workspaces, _ := dataloaders(ctx).Workspace.LoadAll(ids)
	for _, w := range workspaces {
		if w == nil {
			continue
		}
		for _, m := range w.Members {
			if um, ok := m.(*gqlmodel.WorkspaceUserMember); ok && um.UserID == owner {
				return true
			}
		}
	}
	return false
}

// hiddenValue is the empty value of the field's type: "" for a string, an
// empty list, null for a nullable field, and an empty object for a non-null one.
func hiddenValue(ctx context.Context, res any) any {
	if res == nil {
		return nil
	}
	t := reflect.TypeOf(res)
	if t.Kind() == reflect.Pointer {
		if fc := graphql.GetFieldContext(ctx); fc != nil && fc.Field.Definition.Type.NonNull {
			return reflect.New(t.Elem()).Interface()
		}
	}
	return reflect.Zero(t).Interface()
}
