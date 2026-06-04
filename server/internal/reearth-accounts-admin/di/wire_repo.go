package di

import (
	"github.com/goforj/wire"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
)

// repoWire provides the MongoDB client and the shared repository container, and
// exposes the individual repository interfaces consumed by the usecase layer.
var repoWire = wire.NewSet(
	provideMongoClient,
	provideRepoContainer,
	wire.FieldsOf(new(*repo.Container), "User", "Role", "Permittable"),
)
