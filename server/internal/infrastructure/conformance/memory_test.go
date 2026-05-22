package conformance

import (
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
)

func TestMemoryConformance(t *testing.T) {
	Run(t, func(t *testing.T) (*repo.Container, Caps, func()) {
		// memory is an in-process map: no real transactions, no filter
		// enforcement, no ordered FindByIDs, no real pagination, no unique
		// constraints, exact-case email lookup.
		return memory.New(), Caps{}, func() {}
	})
}
