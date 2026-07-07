package internal

import (
	"errors"
	"strconv"
)

// ErrInvalidPageParam is returned by ParsePageParam for a present but invalid
// pagination query parameter.
var ErrInvalidPageParam = errors.New("invalid pagination parameter")

// maxPageParam bounds page / per_page. pagination.ToPagination computes the
// offset as (page-1)*perPage; without an upper bound a huge page could overflow
// int64 and produce a negative offset (bad query / panic). This ceiling is far
// larger than any realistic request yet leaves offset (<= maxPageParam*100) well
// within int64, so values above it are rejected as 400 rather than accepted.
const maxPageParam int64 = 1_000_000_000_000 // 1e12

// ParsePageParam parses a 1-based pagination query parameter (page / per_page).
// An empty value returns 0, the "use the default" sentinel understood by
// pagination.ToPagination. A present but non-numeric, < 1, or absurdly large
// value is an error so client mistakes surface as 400 rather than silently
// falling back to defaults or overflowing the offset computation.
func ParsePageParam(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 1 || n > maxPageParam {
		return 0, ErrInvalidPageParam
	}
	return n, nil
}
