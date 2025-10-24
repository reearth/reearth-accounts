package proxy

import (
	accountid "github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/util"
)

func WorkspaceByIDsResponseTo(r *WorkspaceByIDsResponse, err error) ([]*workspace.Workspace, error) {
	if err != nil || r == nil {
		return nil, err
	}
	return util.TryMap(r.Nodes, WorkspaceByIDsNodeTo)
}

func WorkspaceByIDsNodeTo(r WorkspaceByIDsNodesNode) (*workspace.Workspace, error) {
	if r == nil {
		return nil, nil
	}
	w, ok := r.(*WorkspaceByIDsNodesWorkspace)
	if !ok || w == nil {
		return nil, nil
	}
	return ToWorkspace(w.FragmentWorkspace)
}

func ToWorkspaces(r []FragmentWorkspace) ([]*workspace.Workspace, error) {
	ws := make([]*workspace.Workspace, len(r))
	for i, w := range r {
		wsp, err := ToWorkspace(w)
		if err != nil {
			return nil, err
		}
		ws[i] = wsp
	}
	return ws, nil
}

func ToWorkspace(r FragmentWorkspace) (*workspace.Workspace, error) {
	id, err := workspace.IDFrom(r.Id)
	if err != nil {
		return nil, err
	}
	members := map[accountid.UserID]workspace.Member{}
	integrations := map[accountid.IntegrationID]workspace.Member{}

	for i := range r.Members {
		w, ok := r.Members[i].(*FragmentWorkspaceMembersWorkspaceUserMember)
		if ok {
			id, err := user.IDFrom(w.UserId)
			if err != nil {
				return nil, err
			}

			members[id] = workspace.Member{
				Role: ToRole(w.Role),
			}
		}
		in, ok := r.Members[i].(*FragmentWorkspaceMembersWorkspaceIntegrationMember)
		if ok {
			iid, err := accountid.IntegrationIDFrom(in.IntegrationId)
			if err != nil {
				return nil, err
			}
			var uid user.ID
			if in.InvitedById != "" {
				uid, err = user.IDFrom(in.InvitedById)
				if err != nil {
					return nil, err
				}
			}

			integrations[iid] = workspace.Member{
				Role:      ToRole(in.Role),
				InvitedBy: uid,
			}
		}

	}
	return workspace.New().ID(id).
		Name(r.Name).Personal(r.Personal).Members(members).Integrations(integrations).Build()
}

func ToRole(r Role) workspace.Role {
	switch r {
	case RoleMaintainer:
		return workspace.RoleMaintainer
	case RoleReader:
		return workspace.RoleReader
	case RoleOwner:
		return workspace.RoleOwner
	case RoleWriter:
		return workspace.RoleWriter
	default:
		return workspace.RoleOwner
	}
}
