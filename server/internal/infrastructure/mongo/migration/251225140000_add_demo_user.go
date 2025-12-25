package migration

import (
	"context"
	"crypto/rand"
	"os"
	"strings"

	"github.com/oklog/ulid"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo/mongodoc"
	"go.mongodb.org/mongo-driver/bson"
	"golang.org/x/crypto/bcrypt"
)

// AddDemoUser creates a demo user for mock authentication in E2E tests
func AddDemoUser(ctx context.Context, c DBClient) error {
	// Only run in mock auth mode (e.g., E2E tests)
	if os.Getenv("REEARTH_MOCK_AUTH") != "true" {
		return nil
	}

	userCol := c.Collection("user")
	workspaceCol := c.Collection("workspace")
	permittableCol := c.Collection("permittable")
	roleCol := c.Collection("role")

	demoUserName := "Demo user"
	demoUserEmail := "demo@reearth.io"

	// Check if demo user already exists
	var existingUser bson.M
	err := userCol.Client().FindOne(ctx, bson.M{"name": demoUserName}).Decode(&existingUser)
	if err == nil {
		// Demo user already exists
		return nil
	}

	// Generate IDs
	userID := strings.ToLower(ulid.MustNew(ulid.Now(), rand.Reader).String())
	workspaceID := strings.ToLower(ulid.MustNew(ulid.Now(), rand.Reader).String())

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// Get self role
	var selfRole bson.M
	err = roleCol.Client().FindOne(ctx, bson.M{"name": "self"}).Decode(&selfRole)
	if err != nil {
		return err
	}

	// Get owner role
	var ownerRole bson.M
	err = roleCol.Client().FindOne(ctx, bson.M{"name": "owner"}).Decode(&ownerRole)
	if err != nil {
		return err
	}

	// Create user
	userDoc := mongodoc.UserDocument{
		ID:        userID,
		Name:      demoUserName,
		Email:     demoUserEmail,
		Subs:      []string{userID},
		Workspace: workspaceID,
		Password:  hashedPassword,
		Verification: &mongodoc.UserVerificationDoc{
			Verified: true,
		},
		Metadata: mongodoc.UserMetadataDoc{
			Lang: "ja",
		},
	}

	err = userCol.SaveOne(ctx, userID, &userDoc)
	if err != nil {
		return err
	}

	// Create workspace
	workspaceDoc := mongodoc.WorkspaceDocument{
		ID:       workspaceID,
		Name:     demoUserName,
		Personal: true,
	}

	err = workspaceCol.SaveOne(ctx, workspaceID, &workspaceDoc)
	if err != nil {
		return err
	}

	// Create permittable
	permittableID := strings.ToLower(ulid.MustNew(ulid.Now(), rand.Reader).String())
	permittableDoc := mongodoc.PermittableDocument{
		ID:      permittableID,
		RoleIDs: []string{selfRole["id"].(string)},
		UserID:  userID,
		WorkspaceRoles: []mongodoc.WorkspaceRoleDocument{
			{
				WorkspaceID: workspaceID,
				RoleID:      ownerRole["id"].(string),
			},
		},
	}

	err = permittableCol.SaveOne(ctx, permittableID, &permittableDoc)
	if err != nil {
		return err
	}

	return nil
}
