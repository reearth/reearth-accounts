package gqlmodel

// VisibilityOwner is the user whose data an object carries, for @visibility.
// OwnRecord marks a record returned to its own user before they have a
// session, such as the result of signup.
type VisibilityOwner struct {
	UserID    ID
	OwnRecord bool
}

func (u *User) VisibilityOwner() VisibilityOwner {
	return VisibilityOwner{UserID: u.ID, OwnRecord: u.ownRecord}
}

// Me is only ever built for the viewer.
func (m *Me) VisibilityOwner() VisibilityOwner {
	return VisibilityOwner{UserID: m.ID, OwnRecord: true}
}

func (m *WorkspaceUserMember) VisibilityOwner() VisibilityOwner {
	return VisibilityOwner{UserID: m.UserID}
}
