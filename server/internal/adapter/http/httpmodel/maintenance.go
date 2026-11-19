package httpmodel

import "github.com/reearth/reearth-accounts/server/internal/usecase/maintenance"

// MaintenanceRunResponse counts what one maintenance pass did. Ran is false
// when another pass on the same instance was running.
type MaintenanceRunResponse struct {
	ClaimsExpired  int            `json:"claimsExpired"`
	Deleted        int            `json:"deleted"`
	Failed         int            `json:"failed"`
	Lost           int            `json:"lost"`
	PendingExpired int            `json:"pendingExpired"`
	Ran            bool           `json:"ran"`
	Retained       map[string]int `json:"retained"`
	Retried        int            `json:"retried"`
	Sent           int            `json:"sent"`
}

func NewMaintenanceRunResponse(r maintenance.Report) MaintenanceRunResponse {
	retained := r.Retained
	if retained == nil {
		retained = map[string]int{}
	}
	return MaintenanceRunResponse{
		ClaimsExpired:  r.Sweep.Expired,
		Deleted:        r.Sweep.Deleted,
		Failed:         r.Failed,
		Lost:           r.Lost,
		PendingExpired: r.Sweep.TimedOut,
		Ran:            r.Ran,
		Retained:       retained,
		Retried:        r.Retried,
		Sent:           r.Sent,
	}
}
