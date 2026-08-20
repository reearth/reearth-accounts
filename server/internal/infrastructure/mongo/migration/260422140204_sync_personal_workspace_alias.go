package migration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearthx/mongox"
	"go.mongodb.org/mongo-driver/bson"
)

const (
	// syncPersonalWorkspaceAliasKey is this migration's key in migrations.go.
	// It is recorded on every skip record so the records stay attributable
	// if the migration is re-timestamped again.
	syncPersonalWorkspaceAliasKey int64 = 260422140204

	// aliasSyncSkipMarker prefixes every skipped-workspace log line so the
	// records can be pulled out of Cloud Logging with a single text filter:
	//   gcloud logging read 'textPayload:"ALIAS_SYNC_SKIPPED"'
	aliasSyncSkipMarker = "ALIAS_SYNC_SKIPPED"

	// aliasSyncUpdateMarker does the same for workspaces that were updated.
	aliasSyncUpdateMarker = "ALIAS_SYNC_UPDATED"

	// aliasSyncSkipCollection persists the skip records. Logs expire with the
	// Cloud Logging retention window, so the same records are stored here to
	// stay queryable when the conflicts are resolved manually later. Keyed by
	// workspace ID, so re-running the migration refreshes rather than
	// duplicates each record.
	aliasSyncSkipCollection = "migration_alias_sync_skipped"

	// aliasSyncSkipReason is the only reason a workspace is skipped today.
	// Kept as a field so the records stay filterable if more reasons appear.
	aliasSyncSkipReason = "desired_alias_taken_by_another_workspace"

	aliasSyncReadBatchSize  = 1000
	aliasSyncWriteChunkSize = 500
)

// AliasSyncSkipRecord describes one personal workspace whose alias could not be
// synced to its owner's alias.
type AliasSyncSkipRecord struct {
	ID                     string    `bson:"id" json:"id"`
	MigrationKey           int64     `bson:"migrationkey" json:"migrationKey"`
	WorkspaceID            string    `bson:"workspaceid" json:"workspaceId"`
	WorkspaceAlias         string    `bson:"workspacealias" json:"workspaceAlias"`
	UserID                 string    `bson:"userid" json:"userId"`
	UserAlias              string    `bson:"useralias" json:"userAlias"`
	ConflictingWorkspaceID string    `bson:"conflictingworkspaceid" json:"conflictingWorkspaceId"`
	Reason                 string    `bson:"reason" json:"reason"`
	RecordedAt             time.Time `bson:"recordedat" json:"recordedAt"`
}

type aliasSyncOwner struct {
	userID string
	alias  string
}

type aliasSyncCandidate struct {
	workspaceID string
	alias       string
}

// SyncPersonalWorkspaceAlias copies each user's alias onto their personal
// workspace so the two stay in sync.
//
// The workspace collection carries a unique case-insensitive index on alias
// (alias_case_insensitive_unique, added by AddCaseInsensitiveWorkspaceAliasIndex),
// and user aliases live in a separate namespace from workspace aliases, so a
// user's alias can already be held by an unrelated workspace. Writing it
// anyway aborts the whole migration with E11000, so conflicting workspaces are
// left untouched and reported instead: one JSON line per workspace on stdout,
// plus a durable record in aliasSyncSkipCollection.
func SyncPersonalWorkspaceAlias(ctx context.Context, c DBClient) error {
	userCol := c.Collection("user")
	workspaceCol := c.Collection("workspace")

	// personal workspace ID -> its owner
	owners := map[string]aliasSyncOwner{}
	if err := userCol.Find(ctx, bson.D{}, &mongox.BatchConsumer{
		Size: aliasSyncReadBatchSize,
		Callback: func(rows []bson.Raw) error {
			for _, row := range rows {
				var doc mongodoc.UserDocument
				if err := bson.Unmarshal(row, &doc); err != nil {
					return err
				}
				if doc.Workspace == "" || doc.Alias == "" {
					continue
				}
				owners[doc.Workspace] = aliasSyncOwner{userID: doc.ID, alias: doc.Alias}
			}
			return nil
		},
	}); err != nil {
		return fmt.Errorf("failed to scan users: %w", err)
	}

	// lower(alias) -> workspace ID for *every* workspace, mirroring what
	// alias_case_insensitive_unique enforces, so a desired alias can be checked
	// before it is written. Soft-deleted workspaces are included on purpose:
	// the index covers them too.
	taken := map[string]string{}
	candidates := make([]aliasSyncCandidate, 0)
	if err := workspaceCol.Find(ctx, bson.D{}, &mongox.BatchConsumer{
		Size: aliasSyncReadBatchSize,
		Callback: func(rows []bson.Raw) error {
			for _, row := range rows {
				var doc mongodoc.WorkspaceDocument
				if err := bson.Unmarshal(row, &doc); err != nil {
					return err
				}
				if doc.Alias != "" {
					taken[strings.ToLower(doc.Alias)] = doc.ID
				}
				if doc.Personal {
					candidates = append(candidates, aliasSyncCandidate{workspaceID: doc.ID, alias: doc.Alias})
				}
			}
			return nil
		},
	}); err != nil {
		return fmt.Errorf("failed to scan workspaces: %w", err)
	}

	// Decide everything up front, before any write, so the outcome does not
	// depend on a cursor running concurrently with updates to the same
	// collection. candidates is a slice, so the order — and therefore which
	// workspace wins a contested alias — is deterministic.
	updates := map[string]string{}
	updateOrder := make([]string, 0)
	skips := make([]AliasSyncSkipRecord, 0)
	now := time.Now().UTC()

	for _, cand := range candidates {
		owner, ok := owners[cand.workspaceID]
		if !ok {
			continue
		}
		if cand.alias == owner.alias {
			continue
		}

		desired := strings.ToLower(owner.alias)
		// A holder equal to this workspace means the alias only differs by
		// case, which the index treats as the same value: safe to normalize.
		if holder, exists := taken[desired]; exists && holder != cand.workspaceID {
			skips = append(skips, AliasSyncSkipRecord{
				ID:                     cand.workspaceID,
				MigrationKey:           syncPersonalWorkspaceAliasKey,
				WorkspaceID:            cand.workspaceID,
				WorkspaceAlias:         cand.alias,
				UserID:                 owner.userID,
				UserAlias:              owner.alias,
				ConflictingWorkspaceID: holder,
				Reason:                 aliasSyncSkipReason,
				RecordedAt:             now,
			})
			continue
		}

		// Releasing the old alias is safe because decisions are applied in the
		// order they are made, so a workspace claiming this alias later is
		// always written after this one.
		if cand.alias != "" {
			delete(taken, strings.ToLower(cand.alias))
		}
		taken[desired] = cand.workspaceID
		updates[cand.workspaceID] = owner.alias
		updateOrder = append(updateOrder, cand.workspaceID)
	}

	if err := applyAliasSyncUpdates(ctx, workspaceCol, updateOrder, updates); err != nil {
		return err
	}

	if err := recordAliasSyncSkips(ctx, c, skips); err != nil {
		return err
	}

	fmt.Printf(
		"SyncPersonalWorkspaceAlias: %d personal workspace(s) updated, %d skipped (skip records: %s, log marker: %s)\n",
		len(updateOrder), len(skips), aliasSyncSkipCollection, aliasSyncSkipMarker,
	)
	return nil
}

// applyAliasSyncUpdates writes the new aliases in chunks. Each chunk is read
// fully into memory before it is written, so no cursor is open on the
// workspace collection while it is being modified.
func applyAliasSyncUpdates(ctx context.Context, col *mongox.Collection, order []string, updates map[string]string) error {
	for start := 0; start < len(order); start += aliasSyncWriteChunkSize {
		end := start + aliasSyncWriteChunkSize
		if end > len(order) {
			end = len(order)
		}
		chunk := order[start:end]

		ids := make([]string, 0, len(chunk))
		newRows := make([]interface{}, 0, len(chunk))
		if err := col.Find(ctx, bson.M{"id": bson.M{"$in": chunk}}, &mongox.BatchConsumer{
			Size: aliasSyncWriteChunkSize,
			Callback: func(rows []bson.Raw) error {
				for _, row := range rows {
					var doc mongodoc.WorkspaceDocument
					if err := bson.Unmarshal(row, &doc); err != nil {
						return err
					}
					newAlias, ok := updates[doc.ID]
					if !ok {
						continue
					}
					b, err := json.Marshal(map[string]string{
						"workspaceId": doc.ID,
						"fromAlias":   doc.Alias,
						"toAlias":     newAlias,
					})
					if err != nil {
						return err
					}
					fmt.Printf("%s %s\n", aliasSyncUpdateMarker, b)

					doc.Alias = newAlias
					ids = append(ids, doc.ID)
					newRows = append(newRows, doc)
				}
				return nil
			},
		}); err != nil {
			return fmt.Errorf("failed to load workspaces for alias sync: %w", err)
		}

		if len(ids) == 0 {
			continue
		}
		if err := col.SaveAll(ctx, ids, newRows); err != nil {
			return fmt.Errorf("failed to update personal workspace aliases: %w", err)
		}
	}
	return nil
}

// recordAliasSyncSkips prints every skipped workspace as a single JSON line and
// upserts the same records into aliasSyncSkipCollection.
func recordAliasSyncSkips(ctx context.Context, c DBClient, skips []AliasSyncSkipRecord) error {
	if len(skips) == 0 {
		return nil
	}

	ids := make([]string, 0, len(skips))
	rows := make([]interface{}, 0, len(skips))
	for _, s := range skips {
		b, err := json.Marshal(s)
		if err != nil {
			return fmt.Errorf("failed to marshal alias sync skip record for workspace %s: %w", s.WorkspaceID, err)
		}
		fmt.Printf("%s %s\n", aliasSyncSkipMarker, b)

		ids = append(ids, s.ID)
		rows = append(rows, s)
	}

	if err := c.Collection(aliasSyncSkipCollection).SaveAll(ctx, ids, rows); err != nil {
		return fmt.Errorf("failed to persist alias sync skip records: %w", err)
	}
	return nil
}
