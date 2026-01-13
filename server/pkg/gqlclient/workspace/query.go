package workspace

import (
	"github.com/reearth/reearth-accounts/server/pkg/gqlclient/gqlmodel"
)

type findByUserQuery struct {
	FindByUser []gqlmodel.Workspace `graphql:"findByUser(userId: $userId)"`
}
type findByIDQuery struct {
	Workspace gqlmodel.Workspace `graphql:"findByID(id: $id)"`
}

type findByAliasQuery struct {
	Workspace gqlmodel.Workspace `graphql:"findByAlias(alias: $alias)"`
}

type FindByUserWithPaginationQuery struct {
	FindByUserWithPagination struct {
		Workspaces []gqlmodel.Workspace `graphql:"workspaces"`
		TotalCount int                  `graphql:"totalCount"`
	} `graphql:"findByUserWithPagination(userId: $userId, pagination: {page: $page, size: $size})"`
}

type createWorkspaceMutation struct {
	CreateWorkspace struct {
		Workspace gqlmodel.Workspace
	} `graphql:"createWorkspace(input: {alias: $alias, name: $name, description: $description})"`
}

type updateWorkspaceMutation struct {
	UpdateWorkspace struct {
		Workspace gqlmodel.Workspace
	} `graphql:"updateWorkspace(input: {workspaceId: $workspaceId, name: $name})"`
}

type deleteWorkspaceMutation struct {
	DeleteWorkspace struct {
		WorkspaceId string
	} `graphql:"deleteWorkspace(input: {workspaceId: $workspaceId})"`
}

type addUsersToWorkspaceMutation struct {
	AddUsersToWorkspace struct {
		Workspace gqlmodel.Workspace
	} `graphql:"addUsersToWorkspace(input: {workspaceId: $workspaceId, users: $users})"`
}

type removeUserFromWorkspaceMutation struct {
	RemoveUserFromWorkspace struct {
		Workspace gqlmodel.Workspace
	} `graphql:"removeUserFromWorkspace(input: {workspaceId: $workspaceId, userId: $userId})"`
}

type transferWorkspaceOwnershipMutation struct {
	TransferWorkspaceOwnership struct {
		Workspace gqlmodel.Workspace
	} `graphql:"transferWorkspaceOwnership(input: {workspaceId: $workspaceId, newOwnerId: $newOwnerId})"`
}
