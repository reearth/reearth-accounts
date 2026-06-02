package interactor

import (
	"context"
	"testing"

	accountmemory "github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/mailer"
	"github.com/stretchr/testify/assert"
	"golang.org/x/text/language"
)

// TestUser_SignupOIDC_CIP proves that a CIP-issued, provider-agnostic OIDC param
// provisions a user via the existing SignupOIDC path, and that a second login
// resolves the same user via FindBySub. No CIP-specific production code is needed
// here: the path treats any issuer/sub uniformly.
func TestUser_SignupOIDC_CIP(t *testing.T) {
	ctx := context.Background()
	r := accountmemory.New()

	// SignupOIDC requires RoleSelf and RoleOwner to exist in the DB.
	selfRole := role.New().NewID().Name(interfaces.RoleSelf).MustBuild()
	ownerRole := role.New().NewID().Name(role.RoleOwner.String()).MustBuild()
	assert.NoError(t, r.Role.Save(ctx, *selfRole))
	assert.NoError(t, r.Role.Save(ctx, *ownerRole))

	g := &gateway.Container{Mailer: mailer.NewMock()}
	uc := NewUser(r, g, "", "")

	param := interfaces.SignupOIDCParam{
		Issuer: "https://securetoken.google.com/my-proj",
		Sub:    "cip-uid-123",
		Email:  "cipuser@example.com",
		Name:   "CIP User",
		User: interfaces.SignupUserParam{
			Lang:  &language.English,
			Theme: user.ThemeDefault.Ref(),
		},
	}

	u, err := uc.SignupOIDC(ctx, param)
	assert.NoError(t, err)
	assert.NotNil(t, u)
	assert.True(t, u.ContainAuth(user.AuthFrom("cip-uid-123")))

	// Second login resolves the same user via the provider-agnostic FindBySub path.
	got, err := r.User.FindBySub(ctx, "cip-uid-123")
	assert.NoError(t, err)
	assert.Equal(t, u.ID(), got.ID())
}
