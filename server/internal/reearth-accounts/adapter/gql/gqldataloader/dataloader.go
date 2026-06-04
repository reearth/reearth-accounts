package gqldataloader

//go:generate go run github.com/vektah/dataloaden UserLoader github.com/reearth/reearth-accounts/server/internal/reearth-accounts/adapter/gql/gqlmodel.ID *github.com/reearth/reearth-accounts/server/internal/reearth-accounts/adapter/gql/gqlmodel.User
//go:generate go run github.com/vektah/dataloaden WorkspaceLoader github.com/reearth/reearth-accounts/server/internal/reearth-accounts/adapter/gql/gqlmodel.ID *github.com/reearth/reearth-accounts/server/internal/reearth-accounts/adapter/gql/gqlmodel.Workspace
