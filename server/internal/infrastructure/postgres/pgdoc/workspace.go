package pgdoc

import (
	"encoding/json"
	"time"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/samber/lo"
)

type WorkspaceMetadataJSON struct {
	Description  string `json:"description"`
	Website      string `json:"website"`
	Location     string `json:"location"`
	BillingEmail string `json:"billingemail"`
	PhotoURL     string `json:"photourl"`
}

type WorkspaceMemberRow struct {
	WorkspaceID string
	UserID      string
	Role        string
	InvitedBy   string
	Disabled    bool
}

type WorkspaceIntegrationRow struct {
	WorkspaceID   string
	IntegrationID string
	Role          string
	InvitedBy     string
	Disabled      bool
}

type WorkspaceRow struct {
	ID          string
	Name        string
	Alias       string
	Email       string
	Personal    bool
	Policy      *string
	MembersHash string
	Metadata    []byte // jsonb
	UpdatedAt   time.Time
}

// NewWorkspaceRows decomposes a workspace into the parent row + child rows and
// computes members_hash identically to mongodoc (shared algorithm) so the
// composite (lower(alias), members_hash) unique index matches mongo's behavior.
func NewWorkspaceRows(ws *workspace.Workspace) (*WorkspaceRow, []WorkspaceMemberRow, []WorkspaceIntegrationRow) {
	wid := ws.ID().String()

	membersDoc := map[string]mongodoc.WorkspaceMemberDocument{}
	memberRows := make([]WorkspaceMemberRow, 0)
	for uID, m := range ws.Members().Users() {
		membersDoc[uID.String()] = mongodoc.WorkspaceMemberDocument{Role: string(m.Role), InvitedBy: m.InvitedBy.String(), Disabled: m.Disabled}
		memberRows = append(memberRows, WorkspaceMemberRow{
			WorkspaceID: wid, UserID: uID.String(), Role: string(m.Role), InvitedBy: m.InvitedBy.String(), Disabled: m.Disabled,
		})
	}

	integrationsDoc := map[string]mongodoc.WorkspaceMemberDocument{}
	integRows := make([]WorkspaceIntegrationRow, 0)
	for iID, m := range ws.Members().Integrations() {
		integrationsDoc[iID.String()] = mongodoc.WorkspaceMemberDocument{Role: string(m.Role), InvitedBy: m.InvitedBy.String(), Disabled: m.Disabled}
		integRows = append(integRows, WorkspaceIntegrationRow{
			WorkspaceID: wid, IntegrationID: iID.String(), Role: string(m.Role), InvitedBy: m.InvitedBy.String(), Disabled: m.Disabled,
		})
	}

	membersHash, err := mongodoc.ComputeWorkspaceMembersHash(membersDoc, integrationsDoc)
	if err != nil {
		membersHash = ""
	}

	meta, _ := json.Marshal(WorkspaceMetadataJSON{
		Description:  ws.Metadata().Description(),
		Website:      ws.Metadata().Website(),
		Location:     ws.Metadata().Location(),
		BillingEmail: ws.Metadata().BillingEmail(),
		PhotoURL:     ws.Metadata().PhotoURL(),
	})

	var policy *string
	if p := ws.Policy(); p != nil {
		s := lo.FromPtr(p).String()
		policy = &s
	}

	updatedAt := ws.UpdatedAt()
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}

	return &WorkspaceRow{
		ID: wid, Name: ws.Name(), Alias: ws.Alias(), Email: ws.Email(),
		Personal: ws.IsPersonal(), Policy: policy, MembersHash: membersHash, Metadata: meta, UpdatedAt: updatedAt,
	}, memberRows, integRows
}

func WorkspaceModel(r *WorkspaceRow, members []WorkspaceMemberRow, integrations []WorkspaceIntegrationRow) (*workspace.Workspace, error) {
	tid, err := id.WorkspaceIDFrom(r.ID)
	if err != nil {
		return nil, err
	}

	mems := map[id.UserID]workspace.Member{}
	for _, m := range members {
		uid, err := id.UserIDFrom(m.UserID)
		if err != nil {
			return nil, err
		}
		inviter, err := id.UserIDFrom(m.InvitedBy)
		if err != nil {
			inviter = uid
		}
		mems[uid] = workspace.Member{Role: role.RoleType(m.Role), Disabled: m.Disabled, InvitedBy: inviter}
	}

	integs := map[id.IntegrationID]workspace.Member{}
	for _, m := range integrations {
		iid, err := id.IntegrationIDFrom(m.IntegrationID)
		if err != nil {
			return nil, err
		}
		integs[iid] = workspace.Member{Role: role.RoleType(m.Role), Disabled: m.Disabled, InvitedBy: id.MustUserID(m.InvitedBy)}
	}

	var policy *workspace.PolicyID
	if r.Policy != nil && *r.Policy != "" {
		policy = workspace.PolicyID(*r.Policy).Ref()
	}

	var mj WorkspaceMetadataJSON
	if len(r.Metadata) > 0 {
		if err := json.Unmarshal(r.Metadata, &mj); err != nil {
			return nil, err
		}
	}
	metadata := workspace.MetadataFrom(mj.Description, mj.Website, mj.Location, mj.BillingEmail, mj.PhotoURL)

	return workspace.New().
		ID(tid).Name(r.Name).Alias(r.Alias).Email(r.Email).
		Metadata(metadata).Members(mems).Integrations(integs).
		Personal(r.Personal).Policy(policy).UpdatedAt(r.UpdatedAt).Build()
}
