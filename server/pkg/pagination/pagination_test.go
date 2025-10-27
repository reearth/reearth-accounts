package pagination_test

import (
	"testing"

	"github.com/reearth/reearth-accounts/server/pkg/pagination"
	"github.com/stretchr/testify/assert"
)

func TestPkg_ToPagination(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		p := pagination.ToPagination(1, 0)
		assert.Equal(t, int64(0), p.Offset.Offset)
		assert.Equal(t, int64(50), p.Offset.Limit)
	})

	t.Run("perPage", func(t *testing.T) {
		p := pagination.ToPagination(0, 10)
		assert.Equal(t, int64(0), p.Offset.Offset)
		assert.Equal(t, int64(10), p.Offset.Limit)
	})

	t.Run("maxPerPage", func(t *testing.T) {
		p := pagination.ToPagination(0, 200)
		assert.Equal(t, int64(0), p.Offset.Offset)
		assert.Equal(t, int64(100), p.Offset.Limit)
	})
}
