package internal

import (
	"errors"
	"strconv"
)

// ErrInvalidPageParam is returned by ParsePageParam for a present but invalid
// pagination query parameter.
var ErrInvalidPageParam = errors.New("invalid pagination parameter")

// ParsePageParam parses a 1-based pagination query parameter (page / per_page).
// An empty value returns 0, the "use the default" sentinel understood by
// pagination.ToPagination. A present but non-numeric or < 1 value is an error so
// client mistakes surface as 400 rather than silently falling back to defaults.
func ParsePageParam(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 1 {
		return 0, ErrInvalidPageParam
	}
	return n, nil
}
