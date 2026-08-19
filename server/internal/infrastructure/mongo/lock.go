package mongo

import (
	"context"
	"errors"
	"sync"

	"github.com/avast/retry-go/v4"
	"github.com/google/uuid"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearthx/log"
	"github.com/reearth/reearthx/rerror"
	lock "github.com/square/mongo-lock"
	"go.mongodb.org/mongo-driver/mongo"
)

type Lock struct {
	l      *lock.Client
	hostid string
	locks  sync.Map
}

func NewLock(c *mongo.Collection) (*Lock, error) {
	hostid := uuid.NewString()

	l := lock.NewClient(c)

	return &Lock{
		l:      l,
		hostid: hostid,
	}, nil
}

func (r *Lock) Lock(ctx context.Context, name string) error {
	if r.getLockID(name) != "" {
		return repo.ErrAlreadyLocked
	}

	lockID := uuid.NewString()
	log.Debugfc(ctx, "lock: trying to lock: id=%s, name=%s, host=%s", name, lockID, r.hostid)

	if err := retry.Do(
		func() error { return r.l.XLock(ctx, name, lockID, r.details()) },
		retry.RetryIf(func(err error) bool { return errors.Is(err, lock.ErrAlreadyLocked) }),
	); err != nil {
		log.Debugfc(ctx, "lock: failed to lock: name=%s, id=%s, host=%s, err=%s", name, lockID, r.hostid, err)
		return repo.ErrFailedToLock
	}

	r.setLockID(name, lockID)
	log.Debugfc(ctx, "lock: locked: name=%s, id=%s, host=%s", name, lockID, r.hostid)
	return nil
}

func (r *Lock) Unlock(ctx context.Context, name string) error {
	lockID := r.getLockID(name)
	if lockID == "" {
		return repo.ErrNotLocked
	}

	// Always release the in-process guard, even if the remote unlock below
	// fails, so a canceled caller context or a transient Mongo error doesn't
	// leave this process permanently unable to (re)acquire name: the actual
	// mutual-exclusion guarantee is the Mongo lock document and its TTL, not
	// this local cache. Use a non-cancelable context for the remote call so
	// an already-canceled ctx (e.g. an aborted HTTP request) doesn't turn a
	// routine deferred unlock into a spurious failure.
	defer r.deleteLockID(name)

	if _, err := r.l.Unlock(context.WithoutCancel(ctx), lockID); err != nil {
		return rerror.ErrInternalByWithContext(ctx, err)
	}

	log.Debugfc(ctx, "lock: unlocked: name=%s, id=%s, host=%s", name, lockID, r.hostid)
	return nil
}

func (r *Lock) details() lock.LockDetails {
	if r == nil {
		return lock.LockDetails{}
	}

	return lock.LockDetails{
		Host: r.hostid,
		TTL:  60 * 60, // 1 hour
	}
}

func (r *Lock) setLockID(key, lockID string) {
	if r == nil {
		return
	}

	r.locks.Store(key, lockID)
}

func (r *Lock) getLockID(key string) string {
	if r == nil {
		return ""
	}

	l, ok := r.locks.Load(key)
	if !ok {
		return ""
	}

	return l.(string)
}

func (r *Lock) deleteLockID(key string) {
	if r == nil {
		return
	}

	r.locks.Delete(key)
}
