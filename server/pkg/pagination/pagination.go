package pagination

import "github.com/reearth/reearthx/usecasex"

const maxPerPage = 100
const defaultPerPage int64 = 50

func ToPagination(page, perPage int64) *usecasex.Pagination {
	p := int64(1)
	if page > 0 {
		p = page
	}

	pp := defaultPerPage
	if perPage != 0 {
		if ppr := perPage; 1 <= ppr {
			if ppr > maxPerPage {
				pp = int64(maxPerPage)
			} else {
				pp = ppr
			}
		}
	}

	return usecasex.OffsetPagination{
		Offset: (p - 1) * pp,
		Limit:  pp,
	}.Wrap()
}

type PageResult[T any] struct {
	Items      []T   `json:"items,omitempty"`
	TotalCount int64 `json:"total_count,omitempty"`
	Limit      int64 `json:"limit,omitempty"`
	Offset     int64 `json:"offset,omitempty"`
}
