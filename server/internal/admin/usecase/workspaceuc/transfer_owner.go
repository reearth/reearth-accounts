package workspaceuc

import (
	"context"
	"errors"
	"slices"

	"github.com/reearth/reearth-accounts/server/internal/usecase/permittablesync"
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/rerror"
	"github.com/reearth/reearthx/usecasex"
)

// TransferOwnerUseCase makes an existing member the sole owner of a team
// workspace on behalf of an admin. It is deliberately distinct from the
// user-facing interactor.Workspace.TransferOwnership: there is no operator in
// the workspace, every other owner is demoted (not just the caller), and a
// reader may become the owner.
type TransferOwnerUseCase struct {
	workspaceRepo   workspace.Repo
	userRepo        user.Repo
	roleRepo        role.Repo
	permittableRepo permittable.Repo
	transaction     usecasex.Transaction
}

// NewTransferOwnerUseCase is a Wire provider for TransferOwnerUseCase.
func NewTransferOwnerUseCase(
	workspaceRepo workspace.Repo,
	userRepo user.Repo,
	roleRepo role.Repo,
	permittableRepo permittable.Repo,
	transaction usecasex.Transaction,
) *TransferOwnerUseCase {
	return &TransferOwnerUseCase{
		workspaceRepo:   workspaceRepo,
		userRepo:        userRepo,
		roleRepo:        roleRepo,
		permittableRepo: permittableRepo,
		transaction:     transaction,
	}
}

// TransferOwnerInput is the input for TransferOwnerUseCase.Execute.
type TransferOwnerInput struct {
	WorkspaceID workspace.ID
	NewOwnerID  user.ID
}

// TransferOwnerOutput is the committed result of a transfer.
type TransferOwnerOutput struct {
	Workspace *workspace.Workspace
	// NewOwner is the user record of the new owner.
	NewOwner *user.User
	// DemotedUserIDs are the previous owners, now maintainers, sorted by ID.
	DemotedUserIDs []user.ID
}

// Execute demotes every other owner to maintainer and promotes the target to
// owner. The role changes, their permittables and the workspace are written in
// one transaction, so a failure leaves the workspace's owners unchanged.
func (uc *TransferOwnerUseCase) Execute(ctx context.Context, in TransferOwnerInput) (*TransferOwnerOutput, error) {
	var out *TransferOwnerOutput
	err := usecasex.DoTransaction(ctx, uc.transaction, 0, func(ctx context.Context) error {
		res, err := uc.transfer(ctx, in)
		if err != nil {
			return err
		}
		out = res
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (uc *TransferOwnerUseCase) transfer(ctx context.Context, in TransferOwnerInput) (*TransferOwnerOutput, error) {
	ws, err := uc.workspaceRepo.FindByID(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if ws.IsPersonal() {
		return nil, ErrPersonalWorkspace
	}

	members := ws.Members()
	target := members.User(in.NewOwnerID)
	if target == nil {
		return nil, ErrTargetNotMember
	}
	if target.Disabled {
		return nil, ErrTargetDisabled
	}

	newOwner, err := uc.userRepo.FindByID(ctx, in.NewOwnerID)
	if err != nil {
		if errors.Is(err, rerror.ErrNotFound) {
			return nil, ErrTargetUserNotFound
		}
		return nil, err
	}

	// Disabled owners count too: the end state is exactly one owner.
	demoted := slices.DeleteFunc(members.UsersByRole(role.RoleOwner), func(id user.ID) bool {
		return id == in.NewOwnerID
	})
	if target.Role == role.RoleOwner && len(demoted) == 0 {
		return nil, ErrAlreadyOwner
	}
	slices.SortFunc(demoted, func(a, b user.ID) int { return a.Compare(b) })

	for _, uid := range demoted {
		if err := members.UpdateUserRole(uid, role.RoleMaintainer); err != nil {
			return nil, err
		}
		if err := permittablesync.SetWorkspaceRole(ctx, uc.roleRepo, uc.permittableRepo, uid, ws.ID(), role.RoleMaintainer); err != nil {
			return nil, err
		}
	}

	if target.Role != role.RoleOwner {
		if err := members.UpdateUserRole(in.NewOwnerID, role.RoleOwner); err != nil {
			return nil, err
		}
	}
	if err := permittablesync.SetWorkspaceRole(ctx, uc.roleRepo, uc.permittableRepo, in.NewOwnerID, ws.ID(), role.RoleOwner); err != nil {
		return nil, err
	}

	if err := uc.workspaceRepo.Save(ctx, ws); err != nil {
		return nil, err
	}

	return &TransferOwnerOutput{
		Workspace:      ws,
		NewOwner:       newOwner,
		DemotedUserIDs: demoted,
	}, nil
}
