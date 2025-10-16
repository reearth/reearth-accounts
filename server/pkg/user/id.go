package user

import (
	"github.com/reearth/reearth-accounts/server/pkg/id"
)

type ID = id.UserID
type IDList = id.UserIDList
type WorkspaceID = id.WorkspaceID
type WorkspaceIDList = id.WorkspaceIDList
type IntegrationID = id.IntegrationID
type IntegrationIDList = id.IntegrationIDList

var NewID = id.NewUserID
var MustID = id.MustUserID
var NewWorkspaceID = id.NewWorkspaceID

var IDFrom = id.UserIDFrom
var WorkspaceIDFrom = id.WorkspaceIDFrom

var IDFromRef = id.UserIDFromRef
var WorkspaceIDFromRef = id.WorkspaceIDFromRef

var ErrInvalidID = id.ErrInvalidID
