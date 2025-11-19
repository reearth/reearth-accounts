package mongodoc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// WorkspaceMemberForHash represents a workspace member for hash computation
type WorkspaceMemberForHash struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	InvitedBy string `json:"invited_by"`
	Disabled  bool   `json:"disabled"`
	Type      string `json:"type"` // "user" or "integration"
}

// ComputeWorkspaceMembersHash creates a deterministic hash of workspace members and integrations
// This function is used both in workspace document creation and migration to ensure consistency
func ComputeWorkspaceMembersHash(members, integrations map[string]WorkspaceMemberDocument) (string, error) {
	var allMembers []WorkspaceMemberForHash

	// Add users
	for id, member := range members {
		allMembers = append(allMembers, WorkspaceMemberForHash{
			ID:        id,
			Role:      member.Role,
			InvitedBy: member.InvitedBy,
			Disabled:  member.Disabled,
			Type:      "user",
		})
	}

	// Add integrations
	for id, member := range integrations {
		allMembers = append(allMembers, WorkspaceMemberForHash{
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

	// Convert to JSON and hash
	jsonData, err := json.Marshal(allMembers)
	if err != nil {
		return "", fmt.Errorf("failed to marshal members for hash: %w", err)
	}
	hash := sha256.Sum256(jsonData)
	return hex.EncodeToString(hash[:]), nil
}