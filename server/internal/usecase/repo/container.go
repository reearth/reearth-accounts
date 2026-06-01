package repo

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/config"
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/rerror"
	"github.com/reearth/reearthx/usecasex"
)

// Transactor is the closure-based transaction API the use-case layer should use.
// Implementations begin a transaction, pass a tx-enriched ctx to fn, then commit
// on success or roll back if fn returns an error. Nested calls reuse the
// ambient transaction. Implemented by every backend.
type Transactor interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// TransactorFromTransaction adapts a usecasex.Transaction (Begin/End-shaped)
// into a Transactor (closure-shaped) so backends that already expose the
// usecasex interface get a Transactor for free via usecasex.DoTransaction.
type TransactorFromTransaction struct {
	Tx usecasex.Transaction
}

func (t TransactorFromTransaction) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return usecasex.DoTransaction(ctx, t.Tx, 0, fn)
}

type Container struct {
	User        user.Repo
	Workspace   workspace.Repo
	Role        role.Repo
	Permittable permittable.Repo
	// Transaction is the legacy Begin/End-shaped transaction handle preserved
	// for the public pkg/infrastructure surface. Internal use-case code uses
	// the closure-shaped Transactor below; this field is wired only by
	// backends that expose a usecasex.Transaction (mongo, memory).
	Transaction usecasex.Transaction
	Transactor  Transactor
	Users       []user.Repo
	Config      config.Repo
}

var (
	ErrOperationDenied = rerror.NewE(i18n.T("operation denied"))
)

func (c *Container) Filtered(f workspace.WorkspaceFilter) *Container {
	if c == nil {
		return c
	}
	return &Container{
		Workspace:   c.Workspace.Filtered(f),
		User:        c.User,
		Users:       c.Users,
		Role:        c.Role,
		Permittable: c.Permittable,
		Transaction: c.Transaction,
		Transactor:  c.Transactor,
		Config:      c.Config,
	}
}

// WorkspaceFilterFromOperator creates a WorkspaceFilter from an Operator
func WorkspaceFilterFromOperator(o *workspace.Operator) workspace.WorkspaceFilter {
	return workspace.WorkspaceFilterFromOperator(o)
}
