package workspaceuc

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/reearth/reearth-accounts/server/pkg/membership"
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/log"
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
// one transaction, so a failure leaves the workspace's owners unchanged. With a
// NopTransaction (Mongo without transaction support) the writes are applied one
// by one and a failure part-way leaves them partially applied.
//
// A conflicting concurrent write (a Mongo write conflict, or a Postgres
// deadlock or serialization failure, including one reported at commit)
// surfaces as usecasex.ErrTransaction. It is not retried: a Mongo transaction
// that hit a write conflict is aborted and cannot run fn again.
func (uc *TransferOwnerUseCase) Execute(ctx context.Context, in TransferOwnerInput) (*TransferOwnerOutput, error) {
	if _, nop := uc.transaction.(*usecasex.NopTransaction); nop || uc.transaction == nil {
		log.Warnfc(ctx, "[admin] transfer workspace owner without a transaction; writes are not atomic: workspace=%s", in.WorkspaceID)
	}
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
		if isTransactionConflict(err) {
			return nil, fmt.Errorf("%w: %w", usecasex.ErrTransaction, err)
		}
		return nil, err
	}
	return out, nil
}

func (uc *TransferOwnerUseCase) transfer(ctx context.Context, in TransferOwnerInput) (*TransferOwnerOutput, error) {
	// The workspace lock must be taken by the first read, so the members read
	// are what the previous transfer on this workspace committed. The affected
	// users' permittables are locked after it (lockPermittables); taking the two
	// in the other order can deadlock against another transfer.
	ws, err := uc.workspaceRepo.FindByIDForUpdate(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if ws.IsDeleted() {
		return nil, rerror.ErrNotFound
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
		return nil, &AlreadyOwnerError{Workspace: ws, NewOwner: newOwner}
	}
	slices.SortFunc(demoted, func(a, b user.ID) int { return a.Compare(b) })

	ownerRole, err := uc.roleRepo.FindByName(ctx, role.RoleOwner.String())
	if err != nil {
		return nil, roleLookupError(ctx, role.RoleOwner, err)
	}
	var maintainerRole *role.Role
	if len(demoted) > 0 {
		if maintainerRole, err = uc.roleRepo.FindByName(ctx, role.RoleMaintainer.String()); err != nil {
			return nil, roleLookupError(ctx, role.RoleMaintainer, err)
		}
	}

	affected := append(slices.Clone(demoted), in.NewOwnerID)
	perms, err := uc.lockPermittables(ctx, affected)
	if err != nil {
		return nil, err
	}
	for _, uid := range demoted {
		if err := changeMemberRole(ws, perms, uid, maintainerRole); err != nil {
			return nil, err
		}
	}
	if err := changeMemberRole(ws, perms, in.NewOwnerID, ownerRole); err != nil {
		return nil, err
	}
	for _, uid := range affected {
		if err := uc.permittableRepo.Save(ctx, *perms[uid]); err != nil {
			return nil, err
		}
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

// lockPermittables reads, and on Postgres row-locks, the permittables of the
// users whose role the transfer changes. The permittable holds the user's roles
// in every workspace, so the lock is what keeps a concurrent transfer in
// another workspace that shares one of these users from writing back a stale
// copy. All of the users must be locked in one call, before any is saved: the
// repo takes the locks in user ID order, which is what keeps two transfers with
// crossed users from deadlocking.
func (uc *TransferOwnerUseCase) lockPermittables(ctx context.Context, uids user.IDList) (map[user.ID]*permittable.Permittable, error) {
	found, err := uc.permittableRepo.FindByUserIDsForUpdate(ctx, uids)
	if err != nil && !errors.Is(err, rerror.ErrNotFound) {
		return nil, err
	}
	perms := make(map[user.ID]*permittable.Permittable, len(uids))
	for _, p := range found {
		perms[p.UserID()] = p
	}
	return perms, nil
}

// changeMemberRole changes uid's role in ws and in its entry of perms, adding a
// new Permittable when the user has none. The caller saves both.
func changeMemberRole(ws *workspace.Workspace, perms map[user.ID]*permittable.Permittable, uid user.ID, r *role.Role) error {
	p := perms[uid]
	if p == nil {
		var err error
		if p, err = permittable.NewForUser(uid); err != nil {
			return err
		}
		perms[uid] = p
	}
	return membership.ChangeMemberRole(ws, p, uid, r)
}

// roleLookupError maps a missing role record to ErrRoleNotConfigured so it is
// not reported as a missing workspace.
func roleLookupError(ctx context.Context, name role.RoleType, err error) error {
	if !errors.Is(err, rerror.ErrNotFound) {
		return err
	}
	log.Errorfc(ctx, "[admin] transfer workspace owner: role record %q is missing", name)
	return ErrRoleNotConfigured
}
