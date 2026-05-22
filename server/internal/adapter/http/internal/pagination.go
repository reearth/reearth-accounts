package internal

const (
	defaultPage     = 1
	defaultPageSize = 50
	maxPageSize     = 100
)

// PageParams are offset-pagination query parameters.
type PageParams struct {
	Page     int `query:"page"`
	PageSize int `query:"page_size"`
}

// Normalized returns clamped (page, pageSize) applying defaults and the max bound.
func (p PageParams) Normalized() (page int, pageSize int) {
	page = p.Page
	if page < 1 {
		page = defaultPage
	}
	pageSize = p.PageSize
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
}

// Pagination is the metadata block of a paginated response.
type Pagination struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Total    int `json:"total"`
}

// PageResult is the generic wrapper for paginated list responses.
type PageResult[T any] struct {
	Items      []T        `json:"items"`
	Pagination Pagination `json:"pagination"`
}

// NewPageResult builds a PageResult from items + the request params + total count.
func NewPageResult[T any](items []T, page, pageSize, total int) PageResult[T] {
	if items == nil {
		items = []T{}
	}
	return PageResult[T]{
		Items:      items,
		Pagination: Pagination{Page: page, PageSize: pageSize, Total: total},
	}
}
