package policy

import (
	"time"

	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/policyuc"
	"github.com/reearth/reearth-accounts/server/pkg/legal"
	"github.com/reearth/reearth-accounts/server/pkg/outbox"
)

type PolicyNoticeResponse struct {
	Audience    string    `json:"audience" example:"all"`
	Bump        string    `json:"bump" example:"major"`
	CreatedAt   time.Time `json:"createdAt"`
	Fanout      string    `json:"fanout" enums:"open,complete,covered,withdrawn"`
	ID          string    `json:"id"`
	RequestedBy string    `json:"requestedBy"`
	Type        string    `json:"type" example:"tos"`
	Version     string    `json:"version" example:"2.0.0"`
} // @name PolicyNotice

type CreatePolicyNoticeResponse struct {
	Created bool                 `json:"created"`
	Notice  PolicyNoticeResponse `json:"notice"`
} // @name CreatePolicyNoticeResponse

type PolicyNoticeStatusResponse struct {
	Counts map[string]int       `json:"counts"`
	Notice PolicyNoticeResponse `json:"notice"`
} // @name PolicyNoticeStatus

type ListPolicyNoticesResponse struct {
	Items []PolicyNoticeStatusResponse `json:"items"`
} // @name ListPolicyNoticesResponse

func newPolicyNoticeResponse(n legal.Notice) PolicyNoticeResponse {
	return PolicyNoticeResponse{
		Audience:    string(n.Audience),
		Bump:        string(n.Bump),
		CreatedAt:   n.CreatedAt,
		Fanout:      fanoutState(n.Fanout.End),
		ID:          n.ID.String(),
		RequestedBy: n.RequestedBy.String(),
		Type:        string(n.Ref.Type),
		Version:     n.Ref.Version.String(),
	}
}

func newListPolicyNoticesResponse(ss []policyuc.NoticeStatus) ListPolicyNoticesResponse {
	items := make([]PolicyNoticeStatusResponse, 0, len(ss))
	for _, s := range ss {
		counts := make(map[string]int, len(outbox.States))
		for _, st := range outbox.States {
			counts[string(st)] = s.Counts[st]
		}
		items = append(items, PolicyNoticeStatusResponse{Counts: counts, Notice: newPolicyNoticeResponse(s.Notice)})
	}
	return ListPolicyNoticesResponse{Items: items}
}

func fanoutState(e legal.FanoutEnd) string {
	if e == legal.FanoutOpen {
		return "open"
	}
	return string(e)
}
