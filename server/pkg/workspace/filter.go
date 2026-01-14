package workspace

import (
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"go.mongodb.org/mongo-driver/bson"
)

type WorkspaceFilter struct {
	Readable id.WorkspaceIDList
	Writable id.WorkspaceIDList
}

func WorkspaceFilterFromOperator(o *Operator) WorkspaceFilter {
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
