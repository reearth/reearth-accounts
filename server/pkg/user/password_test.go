package user

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

func TestBcryptPasswordEncoder(t *testing.T) {
	if testing.Short() {
		return
	}

	got, err := (&BcryptPasswordEncoder{}).Encode("abc")
	assert.NoError(t, err)
	err = bcrypt.CompareHashAndPassword(got, []byte("abc"))
	assert.NoError(t, err)

	ok, err := (&BcryptPasswordEncoder{}).Verify("abc", got)
	assert.NoError(t, err)
	assert.True(t, ok)
	ok, err = (&BcryptPasswordEncoder{}).Verify("abcd", got)
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestMockPasswordEncoder(t *testing.T) {
	got, err := (&MockPasswordEncoder{Mock: []byte("ABC")}).Encode("ABC")
	assert.NoError(t, err)
	assert.Equal(t, got, []byte("ABC"))
	got, err = (&MockPasswordEncoder{Mock: []byte("ABC")}).Encode("abc")
	assert.NoError(t, err)
	assert.Equal(t, got, []byte("ABC"))

	ok, err := (&MockPasswordEncoder{Mock: []byte("ABC")}).Verify("ABC", got)
	assert.NoError(t, err)
	assert.True(t, ok)
	ok, err = (&MockPasswordEncoder{Mock: []byte("ABC")}).Verify("abc", got)
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestNoopPasswordEncoder(t *testing.T) {
	got, err := (&NoopPasswordEncoder{}).Encode("abc")
	assert.NoError(t, err)
	assert.Equal(t, got, []byte("abc"))

	ok, err := (&NoopPasswordEncoder{}).Verify("abc", got)
	assert.NoError(t, err)
	assert.True(t, ok)
	ok, err = (&NoopPasswordEncoder{}).Verify("abcd", got)
	assert.NoError(t, err)
	assert.False(t, ok)
}

func Test_ValidatePassword(t *testing.T) {
	tests := []struct {
		name    string
		pass    string
		wantErr bool
	}{
		{
			name:    "should pass",
			pass:    "Abcdafgh1",
			wantErr: false,
		},
		{
			name:    "shouldn't pass: length<8",
			pass:    "Aafgh1",
			wantErr: true,
		},
		{
			name:    "shouldn't pass: don't have numbers",
			pass:    "Abcdefghi",
			wantErr: true,
		},
		{
			name:    "shouldn't pass: don't have upper",
			pass:    "abcdefghi1",
			wantErr: true,
		},
		{
			name:    "shouldn't pass: don't have lower",
			pass:    "ABCDEFGHI1",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(tt *testing.T) {
			out := ValidatePasswordFormat(tc.pass)
			assert.Equal(tt, out != nil, tc.wantErr)
		})
	}
}
