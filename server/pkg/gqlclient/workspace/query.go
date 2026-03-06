package workspace

import (
	"github.com/hasura/go-graphql-client"
	"github.com/reearth/reearth-accounts/server/pkg/gqlclient/gqlmodel"
)

// Query types
type findByUserQuery struct {
	FindByUser []gqlmodel.Workspace `graphql:"findByUser(userId: $userId)"`
}

type findByIDQuery struct {
	Workspace gqlmodel.Workspace `graphql:"findByID(id: $id)"`
}

type findByIDsQuery struct {
	Workspaces []gqlmodel.Workspace `graphql:"findByIDs(ids: $ids)"`
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

// Mutation types with inline workspace fields to avoid union type issues
// Note: Following the pattern from user mutations, we expand input fields inline
// instead of using input: $input to avoid type inference issues
type createWorkspaceMutation struct {
	CreateWorkspace struct {
		Workspace struct {
			ID       graphql.ID     `graphql:"id"`
			Name     graphql.String `graphql:"name"`
			Alias    graphql.String `graphql:"alias"`
			Personal bool           `graphql:"personal"`
		} `graphql:"workspace"`
	} `graphql:"createWorkspace(input: {alias: $alias, name: $name, description: $description})"`
}

type updateWorkspaceMutation struct {
	UpdateWorkspace struct {
		Workspace struct {
			ID       graphql.ID     `graphql:"id"`
			Name     graphql.String `graphql:"name"`
			Alias    graphql.String `graphql:"alias"`
			Personal bool           `graphql:"personal"`
		} `graphql:"workspace"`
	} `graphql:"updateWorkspace(input: {workspaceId: $workspaceId, name: $name, alias: $alias, description: $description, website: $website, photoURL: $photoURL})"`
}

type deleteWorkspaceMutation struct {
	DeleteWorkspace struct {
		WorkspaceID graphql.ID `graphql:"workspaceId"`
	} `graphql:"deleteWorkspace(input: {workspaceId: $workspaceId})"`
}

type addUsersToWorkspaceMutation struct {
	AddUsersToWorkspace struct {
		Workspace struct {
			ID       graphql.ID     `graphql:"id"`
			Name     graphql.String `graphql:"name"`
			Alias    graphql.String `graphql:"alias"`
			Personal bool           `graphql:"personal"`
		} `graphql:"workspace"`
	} `graphql:"addUsersToWorkspace(input: {workspaceId: $workspaceId, users: $users})"`
}

type removeUserFromWorkspaceMutation struct {
	RemoveUserFromWorkspace struct {
		Workspace struct {
			ID       graphql.ID     `graphql:"id"`
			Name     graphql.String `graphql:"name"`
			Alias    graphql.String `graphql:"alias"`
			Personal bool           `graphql:"personal"`
		} `graphql:"workspace"`
	} `graphql:"removeUserFromWorkspace(input: {workspaceId: $workspaceId, userId: $userId})"`
}

type updateUserOfWorkspaceMutation struct {
	UpdateUserOfWorkspace struct {
		Workspace struct {
			ID       graphql.ID     `graphql:"id"`
			Name     graphql.String `graphql:"name"`
			Alias    graphql.String `graphql:"alias"`
			Personal bool           `graphql:"personal"`
		} `graphql:"workspace"`
	} `graphql:"updateUserOfWorkspace(input: $input)"`
}

type addIntegrationToWorkspaceMutation struct {
	AddIntegrationToWorkspace struct {
		Workspace struct {
			ID       graphql.ID     `graphql:"id"`
			Name     graphql.String `graphql:"name"`
			Alias    graphql.String `graphql:"alias"`
			Personal bool           `graphql:"personal"`
		} `graphql:"workspace"`
	} `graphql:"addIntegrationToWorkspace(input: {workspaceId: $workspaceId, integrationId: $integrationId, role: $role})"`
}

type removeIntegrationFromWorkspaceMutation struct {
	RemoveIntegrationFromWorkspace struct {
		Workspace struct {
			ID       graphql.ID     `graphql:"id"`
			Name     graphql.String `graphql:"name"`
			Alias    graphql.String `graphql:"alias"`
			Personal bool           `graphql:"personal"`
		} `graphql:"workspace"`
	} `graphql:"removeIntegrationFromWorkspace(input: {workspaceId: $workspaceId, integrationId: $integrationId})"`
}

type removeIntegrationsFromWorkspaceMutation struct {
	RemoveIntegrationsFromWorkspace struct {
		Workspace struct {
			ID       graphql.ID     `graphql:"id"`
			Name     graphql.String `graphql:"name"`
			Alias    graphql.String `graphql:"alias"`
			Personal bool           `graphql:"personal"`
		} `graphql:"workspace"`
	} `graphql:"removeIntegrationsFromWorkspace(input: {workspaceId: $workspaceId, integrationIds: $integrationIds})"`
}

type updateIntegrationOfWorkspaceMutation struct {
	UpdateIntegrationOfWorkspace struct {
		Workspace struct {
			ID       graphql.ID     `graphql:"id"`
			Name     graphql.String `graphql:"name"`
			Alias    graphql.String `graphql:"alias"`
			Personal bool           `graphql:"personal"`
		} `graphql:"workspace"`
	} `graphql:"updateIntegrationOfWorkspace(input: {workspaceId: $workspaceId, integrationId: $integrationId, role: $role})"`
}

type removeMultipleUsersFromWorkspaceMutation struct {
	RemoveMultipleUsersFromWorkspace struct {
		Workspace struct {
			ID       graphql.ID     `graphql:"id"`
			Name     graphql.String `graphql:"name"`
			Alias    graphql.String `graphql:"alias"`
			Personal bool           `graphql:"personal"`
		} `graphql:"workspace"`
	} `graphql:"removeMultipleUsersFromWorkspace(input: {workspaceId: $workspaceId, userIds: $userIds})"`
}

type transferWorkspaceOwnershipMutation struct {
	TransferWorkspaceOwnership struct {
		Workspace gqlmodel.Workspace
	} `graphql:"transferWorkspaceOwnership(input: {workspaceId: $workspaceId, newOwnerId: $newOwnerId})"`
}
