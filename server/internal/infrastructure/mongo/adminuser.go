package mongo

import (
	"context"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
	"github.com/reearth/reearthx/mongox"
	"github.com/reearth/reearthx/rerror"
	"github.com/reearth/reearthx/usecasex"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type AdminUser struct {
	client *mongox.Collection
}

func NewAdminUser(client *mongox.Client) adminuser.Repo {
	return &AdminUser{client: client.WithCollection("adminuser")}
}

func (r *AdminUser) FindByEmail(ctx context.Context, email string) (*adminuser.AdminUser, error) {
	return r.findOne(ctx, bson.M{"email": adminuser.NormalizeEmail(email)})
}

func (r *AdminUser) FindByID(ctx context.Context, id adminuser.ID) (*adminuser.AdminUser, error) {
	return r.findOne(ctx, bson.M{"id": id.String()})
}

func (r *AdminUser) FindByIDs(ctx context.Context, ids adminuser.IDList) (adminuser.List, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	res, err := r.find(ctx, bson.M{"id": bson.M{"$in": ids.Strings()}})
	if err != nil {
		return nil, err
	}
	return filterAdminUsers(ids, res), nil
}

func (r *AdminUser) List(ctx context.Context, f adminuser.ListFilter) (adminuser.List, *usecasex.PageInfo, error) {
	if f.Pagination != nil && f.Pagination.Cursor != nil {
		return nil, nil, adminuser.ErrCursorPaginationUnsupported
	}

	filter := bson.M{}
	if f.Status != nil {
		filter["status"] = f.Status.String()
	}

	// Sort by id (a ULID) rather than createdat: the ULID's leading bits are the
	// creation timestamp, so this preserves creation order while being unique,
	// giving a deterministic total order for stable offset pagination (createdat
	// alone can tie at millisecond granularity). Matches the {createdat, id}
	// ordering used by the Postgres and in-memory repos.
	sort := &usecasex.Sort{Key: "id"}
	c := mongodoc.NewAdminUserConsumer()
	pageInfo, err := r.client.Paginate(ctx, filter, sort, f.Pagination, c)
	if err != nil {
		return nil, nil, rerror.ErrInternalBy(err)
	}
	return c.Result, pageInfo, nil
}

func (r *AdminUser) Save(ctx context.Context, u *adminuser.AdminUser) error {
	if u == nil {
		return nil
	}
	doc, uid := mongodoc.NewAdminUser(*u)
	if err := r.client.SaveOne(ctx, uid, doc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return adminuser.ErrDuplicatedAdminUser
		}
		return err
	}
	return nil
}

func (r *AdminUser) find(ctx context.Context, filter any) (adminuser.List, error) {
	c := mongodoc.NewAdminUserConsumer()
	if err := r.client.Find(ctx, filter, c); err != nil {
		return nil, err
	}
	return c.Result, nil
}

func (r *AdminUser) findOne(ctx context.Context, filter any) (*adminuser.AdminUser, error) {
	c := mongodoc.NewAdminUserConsumer()
	if err := r.client.FindOne(ctx, filter, c); err != nil {
		return nil, err
	}
	return c.Result[0], nil
}

// filterAdminUsers keeps the order of ids and drops missing ones.
func filterAdminUsers(ids adminuser.IDList, rows adminuser.List) adminuser.List {
	m := make(map[adminuser.ID]*adminuser.AdminUser, len(rows))
	for _, r := range rows {
		if r != nil {
			m[r.ID()] = r
		}
	}
	res := make(adminuser.List, 0, len(ids))
	for _, id := range ids {
		if u, ok := m[id]; ok {
			res = append(res, u)
		}
	}
	return res
}
