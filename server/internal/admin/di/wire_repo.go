//go:build wireinject
// +build wireinject

package di

import (
	"github.com/goforj/wire"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
)

// repoWire provides the repository container (Mongo or Postgres) and exposes
// the individual repository interfaces consumed by the usecase layer.
var repoWire = wire.NewSet(
	provideRepoContainer,
	wire.FieldsOf(new(*repo.Container), "AdminUser", "User", "Workspace", "Role", "Permittable"),
)
