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
	BillingEmail string `json:"billingemail"`
	Description  string `json:"description"`
	Location     string `json:"location"`
	PhotoURL     string `json:"photourl"`
	Website      string `json:"website"`
}

type WorkspaceIntegrationRow struct {
	Disabled      bool
	IntegrationID string
	InvitedBy     string
	Role          string
	WorkspaceID   string
}

type WorkspaceMemberRow struct {
	Disabled    bool
	ExternalID  string
	InvitedBy   string
	Role        string
	UserID      string
	WorkspaceID string
}

type WorkspaceRow struct {
	Alias       string
	Email       string
	ID          string
	MembersHash string
	Metadata    []byte // jsonb
	Name        string
	Personal    bool
	Policy      *string
	UpdatedAt   time.Time
}

type WorkspaceScimConfigRow struct {
	Enabled          bool
	GroupRoleMapping []byte // jsonb
	TokenHash        string
	WorkspaceID      string
}

// NewWorkspaceRows uses mongodoc.ComputeWorkspaceMembersHash so the composite
// (lower(alias), members_hash) unique index behaves identically to mongo.
func NewWorkspaceRows(ws *workspace.Workspace) (*WorkspaceRow, []WorkspaceMemberRow, []WorkspaceIntegrationRow) {
	wid := ws.ID().String()

	membersDoc := map[string]mongodoc.WorkspaceMemberDocument{}
	memberRows := make([]WorkspaceMemberRow, 0)
	for uID, m := range ws.Members().Users() {
		membersDoc[uID.String()] = mongodoc.WorkspaceMemberDocument{
			Disabled:   m.Disabled,
			ExternalID: m.ExternalID,
			InvitedBy:  m.InvitedBy.String(),
			Role:       string(m.Role),
		}
		memberRows = append(memberRows, WorkspaceMemberRow{
			Disabled:    m.Disabled,
			ExternalID:  m.ExternalID,
			InvitedBy:   m.InvitedBy.String(),
			Role:        string(m.Role),
			UserID:      uID.String(),
			WorkspaceID: wid,
		})
	}

	integrationsDoc := map[string]mongodoc.WorkspaceMemberDocument{}
	integRows := make([]WorkspaceIntegrationRow, 0)
	for iID, m := range ws.Members().Integrations() {
		integrationsDoc[iID.String()] = mongodoc.WorkspaceMemberDocument{
			Disabled:  m.Disabled,
			InvitedBy: m.InvitedBy.String(),
			Role:      string(m.Role),
		}
		integRows = append(integRows, WorkspaceIntegrationRow{
			Disabled:      m.Disabled,
			IntegrationID: iID.String(),
			InvitedBy:     m.InvitedBy.String(),
			Role:          string(m.Role),
			WorkspaceID:   wid,
		})
	}

	membersHash, err := mongodoc.ComputeWorkspaceMembersHash(membersDoc, integrationsDoc)
	if err != nil {
		membersHash = ""
	}

	meta, _ := json.Marshal(WorkspaceMetadataJSON{
		BillingEmail: ws.Metadata().BillingEmail(),
		Description:  ws.Metadata().Description(),
		Location:     ws.Metadata().Location(),
		PhotoURL:     ws.Metadata().PhotoURL(),
		Website:      ws.Metadata().Website(),
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
		Alias:       ws.Alias(),
		Email:       ws.Email(),
		ID:          wid,
		MembersHash: membersHash,
		Metadata:    meta,
		Name:        ws.Name(),
		Personal:    ws.IsPersonal(),
		Policy:      policy,
		UpdatedAt:   updatedAt,
	}, memberRows, integRows
}

// ScimConfigRow builds a WorkspaceScimConfigRow from the domain object. Returns
// nil if no SCIM config is set on the workspace.
func ScimConfigRow(ws *workspace.Workspace) *WorkspaceScimConfigRow {
	cfg := ws.ScimConfig()
	if cfg == nil {
		return nil
	}
	grm := cfg.GroupRoleMapping()
	grmStr := make(map[string]string, len(grm))
	for k, v := range grm {
		grmStr[k] = string(v)
	}
	b, _ := json.Marshal(grmStr)
	return &WorkspaceScimConfigRow{
		Enabled:          cfg.Enabled(),
		GroupRoleMapping: b,
		TokenHash:        cfg.TokenHash(),
		WorkspaceID:      ws.ID().String(),
	}
}

func WorkspaceModel(r *WorkspaceRow, members []WorkspaceMemberRow, integrations []WorkspaceIntegrationRow, scimRow *WorkspaceScimConfigRow) (*workspace.Workspace, error) {
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
		mems[uid] = workspace.Member{
			Disabled:   m.Disabled,
			ExternalID: m.ExternalID,
			InvitedBy:  inviter,
			Role:       role.RoleType(m.Role),
		}
	}

	integs := map[id.IntegrationID]workspace.Member{}
	for _, m := range integrations {
		iid, err := id.IntegrationIDFrom(m.IntegrationID)
		if err != nil {
			return nil, err
		}
		// invited_by may be empty/invalid; fall back to zero UserID instead of panicking
		inviter, err := id.UserIDFrom(m.InvitedBy)
		if err != nil {
			inviter = id.UserID{}
		}
		integs[iid] = workspace.Member{
			Disabled:  m.Disabled,
			InvitedBy: inviter,
			Role:      role.RoleType(m.Role),
		}
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

	var scimConfig *workspace.ScimConfig
	if scimRow != nil {
		cfg := workspace.NewScimConfig()
		cfg.SetEnabled(scimRow.Enabled)
		cfg.SetTokenHash(scimRow.TokenHash)
		if len(scimRow.GroupRoleMapping) > 0 {
			var grmStr map[string]string
			if err := json.Unmarshal(scimRow.GroupRoleMapping, &grmStr); err == nil {
				grm := make(map[string]role.RoleType, len(grmStr))
				for k, v := range grmStr {
					grm[k] = role.RoleType(v)
				}
				cfg.SetGroupRoleMapping(grm)
			}
		}
		scimConfig = cfg
	}

	return workspace.New().
		ID(tid).Name(r.Name).Alias(r.Alias).Email(r.Email).
		Metadata(metadata).Members(mems).Integrations(integs).
		Personal(r.Personal).Policy(policy).ScimConfig(scimConfig).UpdatedAt(r.UpdatedAt).Build()
}
