package maintenance

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKey(t *testing.T) {
	off := NewKey("")
	assert.False(t, off.Enabled())
	assert.False(t, off.Matches(""))
	assert.False(t, Key{}.Matches(""))

	k := NewKey("secret")
	assert.True(t, k.Enabled())
	assert.True(t, k.Matches("secret"))
	for _, wrong := range []string{"", "secre", "secret ", "SECRET", "secretsecret"} {
		assert.False(t, k.Matches(wrong), wrong)
	}
	assert.Equal(t, "***", fmt.Sprint(k))
	assert.NotContains(t, fmt.Sprintf("%+v", struct{ K Key }{k}), "secret")
}
