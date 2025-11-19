package mongodoc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
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
	Metadata     WorkspaceMetadataDocument
	Members      map[string]WorkspaceMemberDocument
	Integrations map[string]WorkspaceMemberDocument
	MembersHash  string `bson:"members_hash,omitempty"`
	Personal     bool
	Policy       string `bson:",omitempty"`
}

// computeMembersHash creates a deterministic hash of members and integrations
// for use in compound unique indexes with alias
func computeMembersHash(members map[string]WorkspaceMemberDocument, integrations map[string]WorkspaceMemberDocument) (string, error) {
	// Create a combined structure for consistent hashing
	type memberData struct {
		ID        string `json:"id"`
		Role      string `json:"role"`
		InvitedBy string `json:"invited_by"`
		Disabled  bool   `json:"disabled"`
		Type      string `json:"type"` // "user" or "integration"
	}

	var allMembers []memberData

	for id, member := range members {
		allMembers = append(allMembers, memberData{
			ID:        id,
			Role:      member.Role,
			InvitedBy: member.InvitedBy,
			Disabled:  member.Disabled,
			Type:      "user",
		})
	}

	for id, member := range integrations {
		allMembers = append(allMembers, memberData{
			ID:        id,
			Role:      member.Role,
			InvitedBy: member.InvitedBy,
			Disabled:  member.Disabled,
			Type:      "integration",
		})
	}

	// Sort by ID for deterministic ordering
	sort.Slice(allMembers, func(i, j int) bool {
		return allMembers[i].ID < allMembers[j].ID
	})

	jsonData, err := json.Marshal(allMembers)
	if err != nil {
		return "", fmt.Errorf("failed to marshal members for hash: %w", err)
	}
	hash := sha256.Sum256(jsonData)
	return hex.EncodeToString(hash[:]), nil
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

	metadataDoc := WorkspaceMetadataDocument{
		Description:  ws.Metadata().Description(),
		Website:      ws.Metadata().Website(),
		Location:     ws.Metadata().Location(),
		BillingEmail: ws.Metadata().BillingEmail(),
		PhotoURL:     ws.Metadata().PhotoURL(),
	}

	// Compute members hash for unique indexing
	membersHash, err := computeMembersHash(membersDoc, integrationsDoc)
	if err != nil {
		// In case of marshalling error, fallback to empty hash
		// This should never happen with our data structures, but better to be safe
		membersHash = ""
	}

	wId := ws.ID().String()
	return &WorkspaceDocument{
		ID:           wId,
		Name:         ws.Name(),
		Alias:        ws.Alias(),
		Email:        ws.Email(),
		Metadata:     metadataDoc,
		Members:      membersDoc,
		Integrations: integrationsDoc,
		MembersHash:  membersHash,
		Personal:     ws.IsPersonal(),
		Policy:       lo.FromPtr(ws.Policy()).String(),
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

	metadata := workspace.MetadataFrom(d.Metadata.Description, d.Metadata.Website, d.Metadata.Location, d.Metadata.BillingEmail, d.Metadata.PhotoURL)

	return workspace.New().
		ID(tid).
		Name(d.Name).
		Alias(d.Alias).
		Email(d.Email).
		Metadata(metadata).
		Members(members).
		Integrations(integrations).
		Personal(d.Personal).
		Policy(policy).
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
