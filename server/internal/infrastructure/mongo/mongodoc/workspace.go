package mongodoc

import (
	"github.com/reearth/reearth-accounts/pkg/workspace"
	"github.com/reearth/reearthx/mongox"
	"github.com/samber/lo"
)

type WorkspaceMemberDocument struct {
	Role      string
	InvitedBy string
	Disabled  bool
}

type WorkspaceMetadataDocument struct {
	Description  string
	Website      string
	Location     string
	BillingEmail string
	PhotoURL     string
}

type WorkspaceDocument struct {
	ID           string
	Name         string
	Alias        string
	Email        string
	Metadata     *WorkspaceMetadataDocument
	Members      map[string]WorkspaceMemberDocument
	Integrations map[string]WorkspaceMemberDocument
	Personal     bool
	Policy       string `bson:",omitempty"`
}

func NewWorkspace(ws *workspace.Workspace) (*WorkspaceDocument, string) {
	wId := ws.ID().String()
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

	var metadataDoc *WorkspaceMetadataDocument
	if ws.Metadata() != nil {
		metadataDoc = &WorkspaceMetadataDocument{
			Description:  ws.Metadata().Description(),
			Website:      ws.Metadata().Website(),
			Location:     ws.Metadata().Location(),
			BillingEmail: ws.Metadata().BillingEmail(),
			PhotoURL:     ws.Metadata().PhotoURL(),
		}
	}

	return &WorkspaceDocument{
		ID:           wId,
		Name:         ws.Name(),
		Alias:        ws.Alias(),
		Email:        ws.Email(),
		Metadata:     metadataDoc,
		Members:      membersDoc,
		Integrations: integrationsDoc,
		Personal:     ws.IsPersonal(),
		Policy:       lo.FromPtr(ws.Policy()).String(),
	}, wId
}

func (w *WorkspaceDocument) Model() (*workspace.Workspace, error) {
	tid, err := workspace.IDFrom(w.ID)
	if err != nil {
		return nil, err
	}

	members := map[workspace.UserID]workspace.Member{}
	if w.Members != nil {
		for uId, m := range w.Members {
			uid, err := workspace.UserIDFrom(uId)
			if err != nil {
				return nil, err
			}

			inviterID, err := workspace.UserIDFrom(m.InvitedBy)
			if err != nil {
				return nil, err
			}

			members[uid] = workspace.Member{
				Role:      workspace.Role(m.Role),
				Disabled:  m.Disabled,
				InvitedBy: inviterID,
			}
		}
	}

	integrations := map[workspace.IntegrationID]workspace.Member{}
	if w.Integrations != nil {
		for iId, integrationDoc := range w.Integrations {
			iId, err := workspace.IntegrationIDFrom(iId)
			if err != nil {
				return nil, err
			}
			integrations[iId] = workspace.Member{
				Role:      workspace.Role(integrationDoc.Role),
				Disabled:  integrationDoc.Disabled,
				InvitedBy: workspace.MustUserID(integrationDoc.InvitedBy),
			}
		}
	}

	var policy *workspace.PolicyID
	if w.Policy != "" {
		policy = workspace.PolicyID(w.Policy).Ref()
	}

	var metadata *workspace.Metadata
	if w.Metadata != nil {
		metadata = workspace.MetadataFrom(w.Metadata.Description, w.Metadata.Website, w.Metadata.Location, w.Metadata.BillingEmail, w.Metadata.PhotoURL)
	}

	return workspace.New().
		ID(tid).
		Name(w.Name).
		Alias(w.Alias).
		Email(w.Email).
		Metadata(metadata).
		Members(members).
		Integrations(integrations).
		Personal(w.Personal).
		Policy(policy).
		Build()
}

type WorkspaceConsumer = mongox.SliceFuncConsumer[*WorkspaceDocument, *workspace.Workspace]

func NewWorkspaceConsumer() *WorkspaceConsumer {
	return mongox.NewSliceFuncConsumer[*WorkspaceDocument, *workspace.Workspace](
		func(d *WorkspaceDocument) (*workspace.Workspace, error) {
			m, err := d.Model()
			if err != nil {
				return nil, err
			}
			return m, nil
		},
	)
}
