package mongodoc

import (
	"github.com/reearth/reearth-accounts/pkg/workspace"
	"github.com/reearth/reearthx/account/accountdomain"
	acWorkspace "github.com/reearth/reearthx/account/accountdomain/workspace"
	"github.com/reearth/reearthx/mongox"
)

type WorkspaceMemberDocument struct {
	Role      string
	InvitedBy string
	Disabled  bool
}

type WorkspaceMetadataDocument struct {
	Description string
	Website     string
}

type WorkspaceDocument struct {
	ID           string
	Name         string
	Alias        string
	Email        string
	BillingEmail string
	Metadata     *WorkspaceMetadataDocument
	Members      map[string]WorkspaceMemberDocument
	Integrations map[string]WorkspaceMemberDocument
	Personal     bool
	Location     string `bson:",omitempty"`
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
			Description: ws.Metadata().Description(),
			Website:     ws.Metadata().Website(),
		}
	}

	return &WorkspaceDocument{
		ID:           wId,
		Name:         ws.Name(),
		Alias:        ws.Alias(),
		Email:        ws.Email(),
		BillingEmail: ws.BillingEmail(),
		Metadata:     metadataDoc,
		Members:      membersDoc,
		Integrations: integrationsDoc,
		Personal:     ws.IsPersonal(),
		Location:     ws.LocationOr(""),
	}, wId
}

func (w *WorkspaceDocument) Model() (*workspace.Workspace, error) {
	tid, err := workspace.IDFrom(w.ID)
	if err != nil {
		return nil, err
	}

	members := map[accountdomain.UserID]acWorkspace.Member{}
	if w.Members != nil {
		for uId, m := range w.Members {
			uid, err := accountdomain.UserIDFrom(uId)
			if err != nil {
				return nil, err
			}

			inviterID, err := accountdomain.UserIDFrom(m.InvitedBy)
			if err != nil {
				return nil, err
			}

			members[uid] = acWorkspace.Member{
				Role:      acWorkspace.Role(m.Role),
				Disabled:  m.Disabled,
				InvitedBy: inviterID,
			}
		}
	}

	integrations := map[accountdomain.IntegrationID]acWorkspace.Member{}
	if w.Integrations != nil {
		for iId, integrationDoc := range w.Integrations {
			iId, err := accountdomain.IntegrationIDFrom(iId)
			if err != nil {
				return nil, err
			}
			integrations[iId] = acWorkspace.Member{
				Role:      acWorkspace.Role(integrationDoc.Role),
				Disabled:  integrationDoc.Disabled,
				InvitedBy: accountdomain.MustUserID(integrationDoc.InvitedBy),
			}
		}
	}

	var metadata *workspace.Metadata
	if w.Metadata != nil {
		metadata = workspace.MetadataFrom(w.Metadata.Description, w.Metadata.Website)
	}

	return workspace.New().
		ID(tid).
		Name(w.Name).
		Alias(w.Alias).
		Email(w.Email).
		BillingEmail(w.BillingEmail).
		Metadata(metadata).
		Members(members).
		Integrations(integrations).
		Personal(w.Personal).
		Location(w.Location).
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
