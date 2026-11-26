package interactor

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
)

func TestUser_AccountControlRefusesAClientCallForAUser(t *testing.T) {
	ctx := context.Background()
	uid := user.NewID()
	op := &workspace.Operator{User: &uid, Client: &workspace.Client{ID: "svc", Name: "svc"}}
	uc := NewUser(memory.New(), nil, nil, "", "")

	calls := map[string]func() error{
		"Logout":                    func() error { _, err := uc.Logout(ctx, op); return err },
		"UpdateMe":                  func() error { _, err := uc.UpdateMe(ctx, interfaces.UpdateMeParam{}, op); return err },
		"RemoveMyAuth":              func() error { _, err := uc.RemoveMyAuth(ctx, "auth0", op); return err },
		"DisableMFA":                func() error { return uc.DisableMFA(ctx, op) },
		"EnableMFA":                 func() error { _, err := uc.EnableMFA(ctx, op); return err },
		"GetMFAStatus":              func() error { _, err := uc.GetMFAStatus(ctx, op); return err },
		"RegenerateMFARecoveryCode": func() error { _, err := uc.RegenerateMFARecoveryCode(ctx, op, nil); return err },
		"DeleteMe":                  func() error { return uc.DeleteMe(ctx, uid, op) },
	}
	for name, call := range calls {
		assert.ErrorIs(t, call(), interfaces.ErrInvalidOperator, name)
	}
}
