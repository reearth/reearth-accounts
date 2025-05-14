package mongodoc

import (
	"github.com/reearth/reearth-accounts/pkg/id"
	"github.com/reearth/reearth-accounts/pkg/workspace"
	"github.com/samber/lo"
)

type WorkspaceMemberDocument struct {
	Role      string
	InvitedBy string
	Disabled  bool
}

type WorkspaceDocument struct {
	ID           string
	Name         string
	DisplayName  string
	Members      map[string]WorkspaceMemberDocument
	Integrations map[string]WorkspaceMemberDocument
	Personal     bool
	Policy       string `bson:",omitempty"`
	Location     string `bson:",omitempty"`
}

func NewWorkspace(ws *workspace.Workspace) (*WorkspaceDocument, string) {
	membersDoc := map[string]WorkspaceMemberDocument{}
	for uId, m := range ws.Members().Users() {
		membersDoc[uId.String()] = WorkspaceMemberDocument{
			Role:      string(m.Role),
			Disabled:  m.Disabled,
			InvitedBy: m.InvitedBy.String(),
		}
	}

	integrationsDoc := map[string]WorkspaceMemberDocument{}
	for iId, m := range ws.Members().Integrations() {
		integrationsDoc[iId.String()] = WorkspaceMemberDocument{
			Role:      string(m.Role),
			Disabled:  m.Disabled,
			InvitedBy: m.InvitedBy.String(),
		}
	}

	wId := ws.ID().String()
	return &WorkspaceDocument{
		ID:           wId,
		Name:         ws.Name(),
		Members:      membersDoc,
		Integrations: integrationsDoc,
		Personal:     ws.IsPersonal(),
		Policy:       lo.FromPtr(ws.Policy()).String(),
		Location:     ws.Location(),
	}, wId
}

func (d *WorkspaceDocument) Model() (*workspace.Workspace, error) {
	tid, err := id.WorkspaceIDFrom(d.ID)
	if err != nil {
		return nil, err
	}

	members := map[id.UserID]workspace.Member{}
	if d.Members != nil {
		for uid, member := range d.Members {
			uid, err := id.UserIDFrom(uid)
			if err != nil {
				return nil, err
			}
			inviterID, err := id.UserIDFrom(member.InvitedBy)
			if err != nil {
				inviterID = uid
			}
			members[uid] = workspace.Member{
				Role:      workspace.Role(member.Role),
				Disabled:  member.Disabled,
				InvitedBy: inviterID,
			}
		}
	}

	integrations := map[id.IntegrationID]workspace.Member{}
	if d.Integrations != nil {
		for iId, integrationDoc := range d.Integrations {
			iId, err := id.IntegrationIDFrom(iId)
			if err != nil {
				return nil, err
			}
			integrations[iId] = workspace.Member{
				Role:      workspace.Role(integrationDoc.Role),
				Disabled:  integrationDoc.Disabled,
				InvitedBy: id.MustUserID(integrationDoc.InvitedBy),
			}
		}
	}

	var policy *workspace.PolicyID
	if d.Policy != "" {
		policy = workspace.PolicyID(d.Policy).Ref()
	}

	return workspace.New().
		ID(tid).
		Name(d.Name).
		DisplayName(d.DisplayName).
		Members(members).
		Integrations(integrations).
		Personal(d.Personal).
		Policy(policy).
		Location(d.Location).
		Build()
}

func NewWorkspaces(workspaces []*workspace.Workspace) ([]*WorkspaceDocument, []string) {
	res := make([]*WorkspaceDocument, 0, len(workspaces))
	ids := make([]string, 0, len(workspaces))
	for _, d := range workspaces {
		if d == nil {
			continue
		}
		r, wId := NewWorkspace(d)
		res = append(res, r)
		ids = append(ids, wId)
	}
	return res, ids
}

type WorkspaceConsumer = Consumer[*WorkspaceDocument, *workspace.Workspace]

func NewWorkspaceConsumer() *WorkspaceConsumer {
	return NewConsumer[*WorkspaceDocument, *workspace.Workspace](func(a *workspace.Workspace) bool {
		return true
	})
}
