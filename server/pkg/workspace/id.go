package workspace

import "github.com/reearth/reearth-accounts/pkg/id"

type ID = id.WorkspaceID
type IDList = id.WorkspaceIDList
type UserID = id.UserID
type UserIDList = id.UserIDList
type IntegrationID = id.IntegrationID
type IntegrationIDList = id.IntegrationIDList

var NewID = id.NewWorkspaceID
var NewUserID = id.NewUserID
var NewIntegrationID = id.NewIntegrationID

var IDFrom = id.WorkspaceIDFrom
var UserIDFrom = id.UserIDFrom
var IntegrationIDFrom = id.IntegrationIDFrom

var IDFromRef = id.WorkspaceIDFromRef

var MustUserID = id.MustUserID

var ErrInvalidID = id.ErrInvalidID

type PolicyID string

func (id PolicyID) Ref() *PolicyID {
	return &id
}

func (id PolicyID) String() string {
	return string(id)
}
