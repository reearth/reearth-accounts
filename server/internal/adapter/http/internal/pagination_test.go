package internal_test

import (
	"testing"

	httpinternal "github.com/reearth/reearth-accounts/server/internal/adapter/http/internal"
	"github.com/stretchr/testify/assert"
)

func TestPageParamsNormalized(t *testing.T) {
	p, s := httpinternal.PageParams{Page: 0, PageSize: 0}.Normalized()
	assert.Equal(t, 1, p)
	assert.Equal(t, 50, s)

	p, s = httpinternal.PageParams{Page: 3, PageSize: 999}.Normalized()
	assert.Equal(t, 3, p)
	assert.Equal(t, 100, s)
}

func TestNewPageResult(t *testing.T) {
	r := httpinternal.NewPageResult([]int{1, 2}, 2, 10, 42)
	assert.Equal(t, 2, len(r.Items))
	assert.Equal(t, 42, r.Pagination.Total)
	assert.Equal(t, 2, r.Pagination.Page)
}
