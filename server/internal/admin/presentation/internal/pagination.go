package internal

const (
	defaultPage     = 1
	defaultPageSize = 50
	maxPageSize     = 100
)

// PageParams are offset-pagination query parameters for admin endpoints.
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
