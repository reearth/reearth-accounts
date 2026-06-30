package dregexp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsEmailFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "valid email with standard domain",
			input:    "user@example.com",
			expected: true,
		},
		{
			name:     "valid email with subdomain",
			input:    "user@mail.example.com",
			expected: true,
		},
		{
			name:     "valid email with numbers",
			input:    "user123@example.com",
			expected: true,
		},
		{
			name:     "valid email with special characters",
			input:    "user.name+tag@example.com",
			expected: true,
		},
		{
			name:     "valid email with hyphen in domain",
			input:    "user@my-domain.com",
			expected: true,
		},
		{
			name:     "valid email with long TLD",
			input:    "user@example.museum",
			expected: true,
		},
		{
			name:     "invalid - regular username",
			input:    "regularusername",
			expected: false,
		},
		{
			name:     "invalid - username with spaces",
			input:    "user name",
			expected: false,
		},
		{
			name:     "invalid - no domain",
			input:    "user@",
			expected: false,
		},
		{
			name:     "invalid - no TLD",
			input:    "user@example",
			expected: false,
		},
		{
			name:     "invalid - no local part",
			input:    "@example.com",
			expected: false,
		},
		{
			name:     "invalid - no @ symbol",
			input:    "userexample.com",
			expected: false,
		},
		{
			name:     "invalid - multiple @ symbols",
			input:    "user@@example.com",
			expected: false,
		},
		{
			name:     "invalid - starts with @",
			input:    "@user@example.com",
			expected: false,
		},
		{
			name:     "invalid - ends with @",
			input:    "user@example.com@",
			expected: false,
		},
		{
			name:     "invalid - TLD too short",
			input:    "user@example.c",
			expected: false,
		},
		{
			name:     "invalid - empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "invalid - spaces in email",
			input:    "user @example.com",
			expected: false,
		},
		{
			name:     "invalid - domain with spaces",
			input:    "user@exam ple.com",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsEmailFormat(tt.input)
			assert.Equal(t, tt.expected, result, "IsEmailFormat(%q) = %v, want %v", tt.input, result, tt.expected)
		})
	}
}
