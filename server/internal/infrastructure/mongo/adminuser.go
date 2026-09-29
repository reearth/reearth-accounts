package mongo

import (
	"context"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
	"github.com/reearth/reearthx/mongox"
	"github.com/reearth/reearthx/rerror"
	"github.com/reearth/reearthx/usecasex"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// adminUserSystemAdminGuardLockName is the named distributed lock (backed by
// the same "locks" collection used for config bootstrapping) that serializes
// SaveGuardingLastSystemAdmin's guarded saves, so the "at least one approved
// system_admin" invariant is checked and enforced atomically instead of
// racing another concurrent guarded save.
const adminUserSystemAdminGuardLockName = "adminuser:system_admin_guard"

type AdminUser struct {
	client *mongox.Collection
	lock   repo.Lock
}

func NewAdminUser(client *mongox.Client) adminuser.Repo {
	l, _ := NewLock(client.Database().Collection("locks"))
	return &AdminUser{client: client.WithCollection("adminuser"), lock: l}
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
	if f.Role != nil {
		filter["role"] = f.Role.String()
	}

	// Sort by createdat for creation order; mongox.Paginate automatically
	// appends the unique "id" field as a tie-breaker, so the effective sort is
	// {createdat, id} — deterministic across offset pages even when createdat
	// ties at millisecond granularity. Using createdat as the primary key also
	// lets the {status, createdat} index serve status-filtered listings, and
	// matches the {createdat, id} ordering of the Postgres and in-memory repos.
	sort := &usecasex.Sort{Key: "createdat"}
	c := mongodoc.NewAdminUserConsumer()
	pageInfo, err := r.client.Paginate(ctx, filter, sort, f.Pagination, c)
	if err != nil {
		return nil, nil, rerror.ErrInternalBy(err)
	}
	return c.Result, pageInfo, nil
}

func (r *AdminUser) ExistsApprovedSystemAdminExcept(ctx context.Context, excludeID adminuser.ID) (bool, error) {
	filter := bson.M{
		"status": adminuser.StatusApproved.String(),
		"role":   adminuser.RoleSystemAdmin.String(),
		"id":     bson.M{"$ne": excludeID.String()},
	}
	n, err := r.client.Count(ctx, filter)
	if err != nil {
		return false, rerror.ErrInternalBy(err)
	}
	return n > 0, nil
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

func (r *AdminUser) SaveGuardingLastSystemAdmin(ctx context.Context, u *adminuser.AdminUser, requireOtherSystemAdmin bool) (bool, error) {
	if u == nil {
		return true, nil
	}
	if !requireOtherSystemAdmin {
		return true, r.Save(ctx, u)
	}

	if err := r.lock.Lock(ctx, adminUserSystemAdminGuardLockName); err != nil {
		return false, rerror.ErrInternalByWithContext(ctx, err)
	}
	defer func() { _ = r.lock.Unlock(ctx, adminUserSystemAdminGuardLockName) }()

	exists, err := r.ExistsApprovedSystemAdminExcept(ctx, u.ID())
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	if err := r.Save(ctx, u); err != nil {
		return false, err
	}
	return true, nil
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
