//go:build integration

package conformance

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	mongorepo "github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interactor"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcmongo "github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestMongoConformance(t *testing.T) {
	ctx := context.Background()

	c, err := tcmongo.Run(ctx, "mongo:6")
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Terminate(ctx) })
	dsn, err := c.ConnectionString(ctx)
	require.NoError(t, err)
	cli, err := mongo.Connect(ctx, options.Client().ApplyURI(dsn))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cli.Disconnect(ctx) })

	Run(t, func(t *testing.T) (*repo.Container, Caps, func()) {
		dbName := "conf_" + uuid.NewString()
		db := cli.Database(dbName)

		// case-insensitive unique email index mirrors the production mongo migration
		_, err := db.Collection("user").Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys:    bson.D{{Key: "email", Value: 1}},
			Options: options.Index().SetUnique(true).SetCollation(&options.Collation{Locale: "en", Strength: 2}),
		})
		require.NoError(t, err)

		repos, err := mongorepo.New(ctx, db, false, false, nil)
		require.NoError(t, err)
		return repos, Caps{
			EnforcesFilter:  true,
			EnforcesLimit:   true,
			RealPagination:  true,
			SubstringSearch: true,
			UniqueEmail:     true,
		}, func() { _ = db.Drop(ctx) }
	})

	t.Run("Workspace_LeaveAllAsIntegration_MembersHash", func(t *testing.T) {
		db := cli.Database("conf_" + uuid.NewString())
		t.Cleanup(func() { _ = db.Drop(ctx) })
		repos, err := mongorepo.New(ctx, db, false, false, nil)
		require.NoError(t, err)
		iid, other := id.NewIntegrationID(), id.NewIntegrationID()
		member, _ := seedLeaveAllWorkspaces(t, repos, iid, other)

		_, err = interactor.NewWorkspace(repos, nil, nil).
			LeaveAllAsIntegration(ctx, &workspace.Operator{Integration: &iid, Client: &workspace.Client{ID: "c"}})
		require.NoError(t, err)

		for _, wid := range member {
			var doc mongodoc.WorkspaceDocument
			require.NoError(t, db.Collection("workspace").FindOne(ctx, bson.M{"id": wid.String()}).Decode(&doc))
			assert.NotContains(t, doc.Integrations, iid.String())
			want, err := mongodoc.ComputeWorkspaceMembersHash(doc.Members, doc.Integrations)
			require.NoError(t, err)
			assert.Equal(t, want, doc.MembersHash)
		}
	})

	t.Run("Workspace_LeaveAllAsIntegration_ConcurrentLeaversConverge", func(t *testing.T) {
		db := cli.Database("conf_" + uuid.NewString())
		t.Cleanup(func() { _ = db.Drop(ctx) })
		repos, err := mongorepo.New(ctx, db, false, false, nil)
		require.NoError(t, err)

		const runs = 200
		var lies, failed, retried int
		for range runs {
			uid := id.NewUserID()
			iids := [2]id.IntegrationID{id.NewIntegrationID(), id.NewIntegrationID()}
			ws := workspace.New().NewID().Name("race").Alias("race-" + id.NewWorkspaceID().String()).
				Members(map[id.UserID]workspace.Member{uid: {Role: role.RoleOwner, InvitedBy: uid}}).
				Integrations(map[id.IntegrationID]workspace.Member{
					iids[0]: {Role: role.RoleWriter, InvitedBy: uid},
					iids[1]: {Role: role.RoleReader, InvitedBy: uid},
				}).MustBuild()
			require.NoError(t, repos.Workspace.Save(ctx, ws))

			var results [2]interfaces.LeaveAllResult
			var locks [2]atomic.Int32
			var start, wg sync.WaitGroup
			start.Add(1)
			for n, iid := range iids {
				wg.Add(1)
				go func() {
					defer wg.Done()
					c := *repos
					c.Workspace = &countingLocks{Repo: repos.Workspace, n: &locks[n]}
					start.Wait()
					res, err := interactor.NewWorkspace(&c, nil, nil).
						LeaveAllAsIntegration(ctx, &workspace.Operator{Integration: &iid, Client: &workspace.Client{ID: "c"}})
					assert.NoError(t, err)
					results[n] = res
				}()
			}
			start.Done()
			wg.Wait()

			after, err := repos.Workspace.FindByID(ctx, ws.ID())
			require.NoError(t, err)
			for n, iid := range iids {
				failed += len(results[n].Failed)
				if locks[n].Load() > 1 {
					retried++
				}
				for _, wid := range results[n].Removed {
					if wid == ws.ID() && after.Members().HasIntegration(iid) {
						lies++
					}
				}
			}

			for _, iid := range iids {
				_, err := interactor.NewWorkspace(repos, nil, nil).
					LeaveAllAsIntegration(ctx, &workspace.Operator{Integration: &iid, Client: &workspace.Client{ID: "c"}})
				require.NoError(t, err)
			}
			final, err := repos.Workspace.FindByID(ctx, ws.ID())
			require.NoError(t, err)
			assert.Empty(t, final.Members().Integrations(), "a follow-up call converges")
		}
		t.Logf("runs=%d calls=%d retried=%d failed=%d reported-removed-but-still-member=%d", runs, 2*runs, retried, failed, lies)
	})
}

type countingLocks struct {
	workspace.Repo
	n *atomic.Int32
}

func (r *countingLocks) FindByIDForUpdate(ctx context.Context, wid id.WorkspaceID) (*workspace.Workspace, error) {
	r.n.Add(1)
	return r.Repo.FindByIDForUpdate(ctx, wid)
}
