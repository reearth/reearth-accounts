package postgres

import (
	"context"
	"net/netip"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres/sqlc/gen"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/legal"
	"github.com/reearth/reearthx/rerror"
)

type PolicyAcceptance struct {
	c *Client
}

func NewPolicyAcceptance(c *Client) legal.AcceptanceRepo { return &PolicyAcceptance{c: c} }

func (r *PolicyAcceptance) FindByUser(ctx context.Context, u id.UserID) ([]legal.Acceptance, error) {
	rows, err := r.c.queries(ctx).PolicyAcceptanceFindByUser(ctx, u.String())
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	res := make([]legal.Acceptance, 0, len(rows))
	for _, row := range rows {
		a, err := acceptanceModel(row)
		if err != nil {
			return nil, rerror.ErrInternalByWithContext(ctx, err)
		}
		res = append(res, a)
	}
	return res, nil
}

func (r *PolicyAcceptance) InsertIfAbsent(ctx context.Context, as []legal.Acceptance) error {
	if len(as) == 0 {
		return nil
	}
	var p gen.PolicyAcceptanceInsertParams
	for _, a := range as {
		ip := ""
		if a.Client.IP.IsValid() {
			ip = a.Client.IP.String()
		}
		p.UserIds = append(p.UserIds, a.UserID.String())
		p.Types = append(p.Types, string(a.Type))
		p.Versions = append(p.Versions, a.Version.String())
		p.AcceptedAts = append(p.AcceptedAts, a.AcceptedAt)
		p.Origins = append(p.Origins, a.Client.Origin)
		p.UserAgents = append(p.UserAgents, a.Client.UserAgent)
		p.Ips = append(p.Ips, ip)
	}
	if err := r.c.queries(ctx).PolicyAcceptanceInsert(ctx, p); err != nil {
		return rerror.ErrInternalByWithContext(ctx, err)
	}
	return nil
}

func (r *PolicyAcceptance) RemoveByUser(ctx context.Context, u id.UserID) error {
	if err := r.c.queries(ctx).PolicyAcceptanceRemoveByUser(ctx, u.String()); err != nil {
		return rerror.ErrInternalByWithContext(ctx, err)
	}
	return nil
}

func acceptanceModel(row gen.PolicyAcceptance) (legal.Acceptance, error) {
	u, err := id.UserIDFrom(row.UserID)
	if err != nil {
		return legal.Acceptance{}, err
	}
	v, err := legal.ParseVersion(row.Version)
	if err != nil {
		return legal.Acceptance{}, err
	}
	a := legal.Acceptance{
		AcceptedAt: row.AcceptedAt.UTC(),
		Client:     legal.Client{Origin: row.Origin, UserAgent: row.UserAgent},
		Ref:        legal.Ref{Type: legal.Type(row.Type), Version: v},
		UserID:     u,
	}
	if row.Ip != "" {
		if a.Client.IP, err = netip.ParseAddr(row.Ip); err != nil {
			return legal.Acceptance{}, err
		}
	}
	return a, nil
}
