package legal

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type losingRepo struct{ *fakeRepo }

func (losingRepo) CompareAndSwapHead(context.Context, TypeHead) (bool, error) { return false, nil }

func TestApply_AlwaysLosingCompareAndSetReturnsAnError(t *testing.T) {
	m := Manifest{Timestamp: t0, Entries: []Entry{{Ref: Ref{Type: "tos", Version: Version{Major: 1}}, EffectiveAt: t0}}}

	var err error
	require.NotPanics(t, func() {
		_, err = Apply(context.Background(), losingRepo{newFakeRepo()}, m, func() time.Time { return t0 })
	})
	assert.ErrorContains(t, err, "head compare-and-set lost 64 times")
}
