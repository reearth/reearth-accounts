package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsePageParam(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    int64
		wantErr bool
	}{
		{name: "empty is the default sentinel", in: "", want: 0},
		{name: "one", in: "1", want: 1},
		{name: "typical", in: "42", want: 42},
		{name: "at the upper bound", in: "1000000000000", want: maxPageParam},
		{name: "zero rejected", in: "0", wantErr: true},
		{name: "negative rejected", in: "-1", wantErr: true},
		{name: "non-numeric rejected", in: "abc", wantErr: true},
		{name: "above the upper bound rejected", in: "1000000000001", wantErr: true},
		{name: "overflow value rejected", in: "9223372036854775807", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePageParam(tt.in)
			if tt.wantErr {
				assert.ErrorIs(t, err, ErrInvalidPageParam)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
