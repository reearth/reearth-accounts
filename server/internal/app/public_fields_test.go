package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The anonymous surface is a security boundary. Widening it must change this test.
func TestPublicFields(t *testing.T) {
	assert.Equal(t, map[string]struct{}{
		"authconfig":                   {},
		"createverification":           {},
		"findbyalias":                  {},
		"findbyid":                     {},
		"findbyids":                    {},
		"findusersbyidswithpagination": {},
		"publicworkspace":              {},
		"signup":                       {},
		"signupoidc":                   {},
		"startpasswordreset":           {},
	}, publicFields)
}
