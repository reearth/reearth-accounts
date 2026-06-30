package useruc

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/authz"
	"github.com/stretchr/testify/assert"
)

// A nil operator must be rejected before any repo/authz access, not panic.
func TestListUsersUseCase_Execute_NilOperator(t *testing.T) {
	// nil deps are fine: Execute returns before touching them.
	uc := NewListUsersUseCase(nil, authz.NewChecker(nil, nil, nil))

	out, err := uc.Execute(context.Background(), nil)
	assert.ErrorIs(t, err, ErrInvalidOperator)
	assert.Nil(t, out)
}
