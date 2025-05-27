package workspace

import "github.com/reearth/reearth-accounts/pkg/id"

type ID = id.WorkspaceID
type UserID = id.UserID
type IntegrationID = id.IntegrationID
type IntegrationIDList = id.IntegrationIDList

var IDFrom = id.WorkspaceIDFrom
var UserIDFrom = id.UserIDFrom
var IntegrationIDFrom = id.IntegrationIDFrom

var MustUserID = id.MustUserID

var ErrInvalidID = id.ErrInvalidID

type PolicyID string

func (id PolicyID) Ref() *PolicyID {
	return &id
}

func (id PolicyID) String() string {
	return string(id)
}
