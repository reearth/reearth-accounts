package memory

import (
	"context"
	"sync"

	"github.com/eukarya-inc/reearth-dashboard/pkg/group"
	"github.com/eukarya-inc/reearth-dashboard/pkg/id"
	"github.com/reearth/reearthx/rerror"
)

type Group struct {
	lock sync.Mutex
	data map[id.GroupID]*group.Group
}

func NewGroup() *Group {
	return &Group{
		data: map[id.GroupID]*group.Group{},
	}
}

func NewGroupWith(items ...*group.Group) *Group {
	r := NewGroup()
	ctx := context.Background()
	for _, i := range items {
		_ = r.Save(ctx, *i)
	}
	return r
}

func (g *Group) FindAll(ctx context.Context) (group.List, error) {
	g.lock.Lock()
	defer g.lock.Unlock()

	res := make(group.List, 0, len(g.data))
	for _, v := range g.data {
		res = append(res, v)
	}
	return res, nil
}

func (g *Group) FindByID(ctx context.Context, id id.GroupID) (*group.Group, error) {
	g.lock.Lock()
	defer g.lock.Unlock()

	res, ok := g.data[id]
	if ok {
		return res, nil
	}
	return nil, rerror.ErrNotFound
}

func (g *Group) FindByIDs(ctx context.Context, ids id.GroupIDList) (group.List, error) {
	g.lock.Lock()
	defer g.lock.Unlock()

	res := make(group.List, 0, len(ids))
	for _, id := range ids {
		if v, ok := g.data[id]; ok {
			res = append(res, v)
		}
	}
	return res, nil
}

func (g *Group) Save(ctx context.Context, rl group.Group) error {
	g.lock.Lock()
	defer g.lock.Unlock()

	g.data[rl.ID()] = &rl
	return nil
}

func (g *Group) Remove(ctx context.Context, id id.GroupID) error {
	g.lock.Lock()
	defer g.lock.Unlock()

	delete(g.data, id)
	return nil
}
