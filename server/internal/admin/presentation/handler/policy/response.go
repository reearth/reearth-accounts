package policy

import (
	"time"

	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/policyuc"
	"github.com/reearth/reearth-accounts/server/pkg/legal"
	"github.com/reearth/reearth-accounts/server/pkg/outbox"
)

// PolicyNoticeResponse is one notice. FanoutDone is true once every user of
// the audience has a queued record.
type PolicyNoticeResponse struct {
	Audience    string    `json:"audience" example:"all"`
	Bump        string    `json:"bump" example:"major"`
	CreatedAt   time.Time `json:"createdAt"`
	FanoutDone  bool      `json:"fanoutDone"`
	ID          string    `json:"id"`
	RequestedBy string    `json:"requestedBy"`
	Type        string    `json:"type" example:"tos"`
	Version     string    `json:"version" example:"2.0.0"`
} // @name PolicyNotice

// CreatePolicyNoticeResponse is the notice that was recorded, or the one
// already recorded for the same version and audience when Created is false.
type CreatePolicyNoticeResponse struct {
	Created bool                 `json:"created"`
	Notice  PolicyNoticeResponse `json:"notice"`
} // @name CreatePolicyNoticeResponse

// PolicyNoticeStatusResponse is a notice with the number of its mail records
// in each outbox state (pending, sending, sent, failed, unknown).
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
		FanoutDone:  n.Fanout.Done,
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
