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

// AcceptPoliciesRequest takes the same input as the acceptPolicies mutation.
type AcceptPoliciesRequest struct {
	Policies []PolicyRefRequest `json:"policies" validate:"required,dive"`
}

type PolicyRefRequest struct {
	Type    string `json:"type" validate:"required"`
	Version string `json:"version" validate:"required"`
}

func (r AcceptPoliciesRequest) Refs() ([]legal.Ref, error) {
	res := make([]legal.Ref, len(r.Policies))
	for i, p := range r.Policies {
		t, err := legal.ParseType(p.Type)
		if err != nil {
			return nil, err
		}
		v, err := legal.ParseVersion(p.Version)
		if err != nil {
			return nil, err
		}
		res[i] = legal.Ref{Type: t, Version: v}
	}
	return res, nil
}

// MyPoliciesResponse never carries the client or IP an acceptance recorded.
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

// PolicyVersionResponse has title and url in the user's language, falling
// back to English.
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
