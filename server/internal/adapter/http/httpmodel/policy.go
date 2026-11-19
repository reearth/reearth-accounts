package httpmodel

import (
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/legal"
)

// PolicyWebhookResponse lists the policy types whose newest decision a
// webhook payload became, and the others.
type PolicyWebhookResponse struct {
	Applied []string `json:"applied"`
	Ignored []string `json:"ignored"`
}

func NewPolicyWebhookResponse(r legal.ApplyResult) PolicyWebhookResponse {
	return PolicyWebhookResponse{Applied: typeStrings(r.Applied), Ignored: typeStrings(r.Ignored)}
}

func typeStrings(ts []legal.Type) []string {
	out := make([]string, 0, len(ts))
	for _, t := range ts {
		out = append(out, string(t))
	}
	return out
}

type AcceptPoliciesRequest struct {
	Policies []PolicyRefRequest `json:"policies" validate:"required,dive"`
}

type PolicyRefRequest struct {
	Type    string `json:"type" validate:"required"`
	Version string `json:"version" validate:"required"`
}

func (r AcceptPoliciesRequest) Inputs() []legal.RefInput {
	res := make([]legal.RefInput, len(r.Policies))
	for i, p := range r.Policies {
		res[i] = legal.RefInput{Type: p.Type, Version: p.Version}
	}
	return res
}

type MyPoliciesResponse struct {
	Acceptances []PolicyAcceptanceResponse `json:"acceptances"`
	Pending     []PolicyVersionResponse    `json:"pending"`
	Upcoming    []PolicyVersionResponse    `json:"upcoming"`
}

type PolicyAcceptanceResponse struct {
	AcceptedAt time.Time `json:"acceptedAt"`
	Type       string    `json:"type"`
	Version    string    `json:"version"`
}

type PolicyVersionResponse struct {
	EffectiveDate time.Time `json:"effectiveDate"`
	Required      bool      `json:"required"`
	Title         string    `json:"title"`
	Type          string    `json:"type"`
	URL           string    `json:"url,omitempty"`
	Version       string    `json:"version"`
}

func NewMyPoliciesResponse(ov legal.Overview, lang string) MyPoliciesResponse {
	res := MyPoliciesResponse{
		Acceptances: make([]PolicyAcceptanceResponse, 0, len(ov.Acceptances)),
		Pending:     policyVersionResponses(ov.Pending, lang),
		Upcoming:    policyVersionResponses(ov.Upcoming, lang),
	}
	for _, a := range ov.Acceptances {
		res.Acceptances = append(res.Acceptances, PolicyAcceptanceResponse{AcceptedAt: a.AcceptedAt, Type: string(a.Type), Version: a.Version.String()})
	}
	return res
}

func policyVersionResponses(vs []legal.VersionRecord, lang string) []PolicyVersionResponse {
	res := make([]PolicyVersionResponse, 0, len(vs))
	for _, v := range vs {
		res = append(res, PolicyVersionResponse{
			EffectiveDate: v.EffectiveAt,
			Required:      v.Required,
			Title:         v.Title.Pick(lang),
			Type:          string(v.Type),
			URL:           v.URL.Pick(lang),
			Version:       v.Version.String(),
		})
	}
	return res
}
