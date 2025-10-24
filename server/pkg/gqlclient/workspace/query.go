package workspace

import (
	"github.com/reearth/reearth-accounts/server/pkg/gqlclient/gqlmodel"
)

type findByUserQuery struct {
	FindByUser []gqlmodel.Workspace `graphql:"findByUser(userId: $userId)"`
}
