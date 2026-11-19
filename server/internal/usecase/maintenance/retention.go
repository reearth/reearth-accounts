package maintenance

import (
	"context"
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/legal"
	"github.com/reearth/reearth-accounts/server/pkg/user"
)

func OrphanAcceptances(acceptances legal.AcceptanceRepo, users user.Repo, page int) Retention {
	return &orphanAcceptances{acceptances: acceptances, page: page, users: users}
}

type orphanAcceptances struct {
	acceptances legal.AcceptanceRepo
	after       *id.UserID
	page        int
	users       user.Repo
}

func (*orphanAcceptances) Name() string { return "orphan_policy_acceptances" }

func (h *orphanAcceptances) Run(ctx context.Context, _ time.Time) (int, error) {
	ids, err := h.acceptances.FindUsersAfter(ctx, h.after, h.page)
	if err != nil || len(ids) == 0 {
		h.after = nil
		return 0, err
	}
	found, err := h.users.FindByIDs(ctx, ids)
	if err != nil {
		return 0, err
	}
	removed := 0
	for i, u := range found {
		if u != nil {
			continue
		}
		if err := h.acceptances.RemoveByUser(ctx, ids[i]); err != nil {
			return removed, err
		}
		removed++
	}
	h.after = &ids[len(ids)-1]
	return removed, nil
}
