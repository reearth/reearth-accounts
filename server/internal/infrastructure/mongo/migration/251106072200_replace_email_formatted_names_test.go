package migration

import (
	"context"
	"regexp"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearthx/mongox"
	"github.com/reearth/reearthx/mongox/mongotest"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

func TestReplaceEmailFormattedNames(t *testing.T) {
	ctx := context.Background()
	db := mongotest.Connect(t)(t)

	client := mongox.NewClientWithDatabase(db)
	col := client.WithCollection("user")

	// Setup test data
	testUsers := []mongodoc.UserDocument{
		{
			ID:        "user1",
			Name:      "test@example.com",
			Email:     "test@example.com",
			Alias:     "testalias1",
			Workspace: "workspace1",
		},
		{
			ID:        "user2",
			Name:      "another@domain.org",
			Email:     "another@domain.org",
			Alias:     "testalias2",
			Workspace: "workspace2",
		},
		{
			ID:        "user3",
			Name:      "validusername",
			Email:     "valid@example.com",
			Alias:     "testalias3",
			Workspace: "workspace3",
		},
		{
			ID:        "user4",
			Name:      "user.name@test.co.uk",
			Email:     "user.name@test.co.uk",
			Alias:     "testalias4",
			Workspace: "workspace4",
		},
	}

	for _, user := range testUsers {
		_, err := col.Client().InsertOne(ctx, user)
		assert.NoError(t, err)
	}

	// Run migration
	err := ReplaceEmailFormattedNames(ctx, client)
	assert.NoError(t, err)

	// Verify results
	var results []mongodoc.UserDocument
	cursor, err := col.Client().Find(ctx, bson.M{})
	assert.NoError(t, err)
	err = cursor.All(ctx, &results)
	assert.NoError(t, err)

	userPattern := regexp.MustCompile(`^user-[a-f0-9]{8}$`)
	seenNames := make(map[string]bool)

	for _, result := range results {
		// Check if original email-formatted names were replaced
		assert.NotContains(t, result.Name, "@", "Name should not contain @ symbol")

		switch result.ID {
		case "user1", "user2", "user4":
			// These had email-formatted names, should now be user-{string}
			assert.Regexp(t, userPattern, result.Name, "Name should match user-{string} pattern")
			// Verify uniqueness
			assert.False(t, seenNames[result.Name], "Each generated name should be unique")
			seenNames[result.Name] = true
		case "user3":
			// This had a valid username, should remain unchanged
			assert.Equal(t, "validusername", result.Name)
		}
	}

	// Verify all generated names are unique
	assert.Equal(t, 3, len(seenNames), "Should have 3 unique generated names")
}

func TestGenerateUniqueName(t *testing.T) {
	seenNames := make(map[string]bool)

	// Generate multiple names and ensure uniqueness
	names := make(map[string]bool)
	for i := 0; i < 100; i++ {
		name := generateUniqueName(seenNames)
		assert.NotContains(t, names, name, "Generated name should be unique")
		assert.Regexp(t, `^user-[a-f0-9]{8}$`, name, "Name should match expected pattern")
		names[name] = true
		seenNames[name] = true
	}

	assert.Equal(t, 100, len(names), "Should have generated 100 unique names")
}

func TestGenerateRandomString(t *testing.T) {
	length := 8
	str := generateRandomString(length)
	assert.Equal(t, length, len(str), "Generated string should have correct length")
	assert.Regexp(t, `^[a-f0-9]+$`, str, "Generated string should be hex")
}

func TestEmailRegex(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"test@example.com", true},
		{"user.name@test.co.uk", true},
		{"another+tag@domain.org", true},
		{"valid_email@sub.domain.com", true},
		{"validusername", false},
		{"not-an-email", false},
		{"missing@domain", false},
		{"@nodomain.com", false},
		{"noat.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := emailRegex.MatchString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
