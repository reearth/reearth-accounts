package conformance

import (
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
)

func TestMemoryConformance(t *testing.T) {
	Run(t, func(t *testing.T) (*repo.Container, Caps, func()) {
		return memory.New(), Caps{}, func() {}
	})
}
