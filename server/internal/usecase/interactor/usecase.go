package interactor

import (
	"context"

	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/usecasex"
)

type uc struct {
	tx                     bool
	readableWorkspaces     id.WorkspaceIDList
	writableWorkspaces     id.WorkspaceIDList
	maintainableWorkspaces id.WorkspaceIDList
	ownableWorkspaces      id.WorkspaceIDList
}

func Usecase() *uc {
	return &uc{}
}

func (u *uc) WithReadableWorkspaces(ids ...id.WorkspaceID) *uc {
	u.readableWorkspaces = id.WorkspaceIDList(ids).Clone()
	return u
}

func (u *uc) WithWritableWorkspaces(ids ...id.WorkspaceID) *uc {
	u.writableWorkspaces = id.WorkspaceIDList(ids).Clone()
	return u
}

func (u *uc) WithMaintainableWorkspaces(ids ...id.WorkspaceID) *uc {
	u.maintainableWorkspaces = id.WorkspaceIDList(ids).Clone()
	return u
}

func (u *uc) WithOwnableWorkspaces(ids ...id.WorkspaceID) *uc {
	u.ownableWorkspaces = id.WorkspaceIDList(ids).Clone()
	return u
}

func (u *uc) Transaction() *uc {
	u.tx = true
	return u
}

func Run0(ctx context.Context, op *workspace.Operator, r *repo.Container, e *uc, f func(ctx context.Context) error) (err error) {
	_, _, _, err = Run3(
		ctx, op, r, e,
		func(ctx context.Context) (_, _, _ any, err error) {
			err = f(ctx)
			return
		})
	return
}

func Run1[A any](ctx context.Context, op *workspace.Operator, r *repo.Container, e *uc, f func(ctx context.Context) (A, error)) (a A, err error) {
	a, _, _, err = Run3(
		ctx, op, r, e,
		func(ctx context.Context) (a A, _, _ any, err error) {
			a, err = f(ctx)
			return
		})
	return
}

func Run2[A, B any](ctx context.Context, op *workspace.Operator, r *repo.Container, e *uc, f func(ctx context.Context) (A, B, error)) (a A, b B, err error) {
	a, b, _, err = Run3(
		ctx, op, r, e,
		func(ctx context.Context) (a A, b B, _ any, err error) {
			a, b, err = f(ctx)
			return
		})
	return
}

func Run3[A, B, C any](ctx context.Context, op *workspace.Operator, r *repo.Container, e *uc, f func(ctx context.Context) (A, B, C, error)) (a A, b B, c C, err error) {
	var tr usecasex.Transaction
	if e.tx {
		tr = r.Transaction
	}

	return usecasex.Run3(ctx, f, usecasex.TxUsecase{Transaction: tr}.UseTx(), e.EnsurePermission(op))
}

func (u *uc) EnsurePermission(op *workspace.Operator) usecasex.Middleware {
	return func(next usecasex.MiddlewareHandler) usecasex.MiddlewareHandler {
		return func(ctx context.Context) (context.Context, error) {
			if err := u.checkPermission(op); err != nil {
				return ctx, err
			}
			return next(ctx)
		}
	}
}

func (u *uc) checkPermission(op *workspace.Operator) error {
	ok := true
	if op != nil {
		if u.readableWorkspaces != nil {
			ok = op.IsReadableWorkspace(u.readableWorkspaces...)
		}
		if ok && u.writableWorkspaces != nil {
			ok = op.IsWritableWorkspace(u.writableWorkspaces...)
		}
		if ok && u.maintainableWorkspaces != nil {
			ok = op.IsMaintainingWorkspace(u.maintainableWorkspaces...)
		}
		if ok && u.ownableWorkspaces != nil {
			ok = op.IsOwningWorkspace(u.ownableWorkspaces...)
		}
	}
	if !ok {
		return interfaces.ErrOperationDenied
	}
	return nil
}
