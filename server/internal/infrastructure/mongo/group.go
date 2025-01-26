package mongo

import (
	"context"

	"github.com/eukarya-inc/reearth-dashboard/internal/infrastructure/mongo/mongodoc"
	"github.com/eukarya-inc/reearth-dashboard/pkg/group"
	"github.com/eukarya-inc/reearth-dashboard/pkg/id"
	"github.com/reearth/reearthx/mongox"
	"go.mongodb.org/mongo-driver/bson"
)

var (
	groupIndexes       = []string{}
	groupUniqueIndexes = []string{"id", "name"}
)

type Group struct {
	client *mongox.ClientCollection
}

func NewGroup(client *mongox.Client) *Group {
	return &Group{
		client: client.WithCollection("group"),
	}
}

func (r *Group) Init(ctx context.Context) error {
	return createIndexes(ctx, r.client, groupIndexes, groupUniqueIndexes)
}

func (r *Group) FindAll(ctx context.Context) (group.List, error) {
	filter := bson.M{}
	return r.find(ctx, filter)
}

func (r *Group) FindByID(ctx context.Context, id id.GroupID) (*group.Group, error) {
	return r.findOne(ctx, bson.M{
		"id": id.String(),
	})
}

func (r *Group) FindByIDs(ctx context.Context, ids id.GroupIDList) (group.List, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	filter := bson.M{
		"id": bson.M{
			"$in": ids.Strings(),
		},
	}
	return r.find(ctx, filter)
}

func (r *Group) Save(ctx context.Context, group group.Group) error {
	doc, gId := mongodoc.NewGroup(group)
	return r.client.SaveOne(ctx, gId, doc)
}

func (r *Group) Remove(ctx context.Context, id id.GroupID) error {
	return r.client.RemoveOne(ctx, bson.M{"id": id.String()})
}

func (r *Group) find(ctx context.Context, filter any) (group.List, error) {
	c := mongodoc.NewGroupConsumer()
	if err := r.client.Find(ctx, filter, c); err != nil {
		return nil, err
	}
	if len(c.Result) == 0 {
		return group.List{}, nil
	}
	return (group.List)(c.Result), nil
}

func (r *Group) findOne(ctx context.Context, filter any) (*group.Group, error) {
	c := mongodoc.NewGroupConsumer()
	if err := r.client.FindOne(ctx, filter, c); err != nil {
		return nil, err
	}
	return c.Result[0], nil
}
