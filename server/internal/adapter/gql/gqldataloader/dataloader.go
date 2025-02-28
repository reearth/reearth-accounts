package gqldataloader

//go:generate go run github.com/vektah/dataloaden UserLoader github.com/reearth/reearth-accounts/internal/adapter/gql/gqlmodel.ID *github.com/reearth/reearth-accounts/internal/adapter/gql/gqlmodel.User
//go:generate go run github.com/vektah/dataloaden WorkspaceLoader github.com/reearth/reearth-accounts/internal/adapter/gql/gqlmodel.ID *github.com/reearth/reearth-accounts/internal/adapter/gql/gqlmodel.Workspace
