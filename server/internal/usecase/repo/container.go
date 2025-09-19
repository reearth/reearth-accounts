package repo

import (
	"github.com/reearth/reearth-accounts/internal/usecase"
	"github.com/reearth/reearth-accounts/pkg/id"
	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/rerror"
	"github.com/reearth/reearthx/usecasex"
	"go.mongodb.org/mongo-driver/bson"
)

type Container struct {
	User        User
	Workspace   Workspace
	Role        Role
	Permittable Permittable
	Transaction usecasex.Transaction
	Users       []User
	Config      Config
}

var (
	ErrOperationDenied = rerror.NewE(i18n.T("operation denied"))
)

func (c *Container) Filtered(workspace WorkspaceFilter) *Container {
	if c == nil {
		return c
	}
	return &Container{
		Workspace:   c.Workspace.Filtered(workspace),
		User:        c.User,
		Users:       c.Users,
		Role:        c.Role,
		Permittable: c.Permittable,
		Transaction: c.Transaction,
	}
}

type WorkspaceFilter struct {
	Readable id.WorkspaceIDList
	Writable id.WorkspaceIDList
}

func WorkspaceFilterFromOperator(o *usecase.Operator) WorkspaceFilter {
	return WorkspaceFilter{
		Readable: o.AllReadableWorkspaces(),
		Writable: o.AllWritableWorkspaces(),
	}
}

func (f WorkspaceFilter) Clone() WorkspaceFilter {
	return WorkspaceFilter{
		Readable: f.Readable.Clone(),
		Writable: f.Writable.Clone(),
	}
}

func (f WorkspaceFilter) Merge(g WorkspaceFilter) WorkspaceFilter {
	var r, w id.WorkspaceIDList
	if f.Readable != nil || g.Readable != nil {
		if f.Readable == nil {
			r = g.Readable.Clone()
		} else {
			r = append(f.Readable, g.Readable...)
		}
	}
	if f.Writable != nil || g.Writable != nil {
		if f.Writable == nil {
			w = g.Writable.Clone()
		} else {
			w = append(f.Writable, g.Writable...)
		}
	}
	return WorkspaceFilter{
		Readable: r,
		Writable: w,
	}
}

func (f WorkspaceFilter) CanRead(id id.WorkspaceID) bool {
	return f.Readable == nil || f.Readable.Has(id) || f.CanWrite(id)
}

func (f WorkspaceFilter) CanWrite(id id.WorkspaceID) bool {
	return len(f.Writable) == 0 || f.Writable.Has(id)
}

func (f WorkspaceFilter) Filter(q any) any {
	if f.Readable == nil {
		return q
	}

	return bson.M{
		"$and": bson.A{
			bson.M{"id": bson.M{"$in": f.Readable.Strings()}},
			q,
		},
	}
}
