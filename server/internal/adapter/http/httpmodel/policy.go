package httpmodel

import "github.com/reearth/reearth-accounts/server/pkg/legal"

// PolicyWebhookResponse lists the policy types a webhook payload changed and
// the ones it left as they were (an older payload or a replay).
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
