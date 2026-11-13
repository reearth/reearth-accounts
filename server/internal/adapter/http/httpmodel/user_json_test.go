package httpmodel

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUserResponse_JSON(t *testing.T) {
	uid := user.MustID("01hzzzzzzzzzzzzzzzzzzzzzzz")
	wid := id.MustWorkspaceID("01hyyyyyyyyyyyyyyyyyyyyyyy")
	deletedAt := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	u := user.New().ID(uid).Name("alice").Alias("al").Email("alice@example.com").Workspace(wid).
		Auths([]user.Auth{{Provider: "auth0", Sub: "auth0|alice"}}).
		Metadata(user.MetadataFrom("users/a.png", "desc", "https://a.example", user.NewMetadata().Lang(), user.ThemeDark)).
		Verification(user.VerificationFrom("code", time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), true)).
		DeletedAt(&deletedAt).
		MustBuild().WithHost("a.example")

	got, err := json.Marshal(NewUserResponse(u, UnrestrictedViewer()))
	require.NoError(t, err)
	assert.Equal(t, `{"id":"01hzzzzzzzzzzzzzzzzzzzzzzz","name":"alice","alias":"al","email":"alice@example.com",`+
		`"host":"a.example","workspace":"01hyyyyyyyyyyyyyyyyyyyyyyy","auths":["auth0"],`+
		`"metadata":{"description":"desc","website":"https://a.example","photo_url":"users/a.png","lang":"und","theme":"dark"},`+
		`"verification":{"expiration":"2030-01-01T00:00:00.000Z","verified":true},`+
		`"is_deleted":true,"deleted_at":"2026-02-03T04:05:06Z"}`, string(got))
}
