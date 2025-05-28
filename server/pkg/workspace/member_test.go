package workspace

import (
	"testing"

	"github.com/reearth/reearth-accounts/pkg/user"
	"github.com/stretchr/testify/assert"
)

func TestNewMembers(t *testing.T) {
	m := NewMembers()
	assert.Equal(t, &Members{
		users:        map[UserID]Member{},
		integrations: map[IntegrationID]Member{},
		fixed:        false,
	}, m)
}
func TestNewMembersWith(t *testing.T) {
	uid := NewUserID()
	m := NewMembersWith(map[UserID]Member{uid: {Role: RoleOwner}}, nil, true)
	assert.Equal(t, &Members{
		users:        map[UserID]Member{uid: {Role: RoleOwner}},
		integrations: nil,
		fixed:        true,
	}, m)
}

func TestInitMembers(t *testing.T) {
	uid := NewUserID()
	m := InitMembers(uid)
	assert.Equal(t, &Members{
		users:        map[UserID]Member{uid: {Role: RoleOwner, InvitedBy: uid}},
		integrations: nil,
		fixed:        true,
	}, m)
}

func TestMembers_Clone(t *testing.T) {
	m := &Members{
		users: map[UserID]Member{
			NewUserID(): {Role: RoleOwner},
		},
		integrations: map[IntegrationID]Member{
			NewIntegrationID(): {Role: RoleOwner},
		},
		fixed: true,
	}
	m2 := m.Clone()
	assert.Equal(t, m, m2)
	assert.NotSame(t, m, m2)
	assert.Nil(t, (*Members)(nil).Clone())
}

func TestMembers_Users(t *testing.T) {
	uid := NewUserID()
	u := &Members{users: map[UserID]Member{uid: {Role: RoleOwner}}}
	assert.Equal(t, map[UserID]Member{uid: {Role: RoleOwner}}, u.Users())
	assert.Equal(t, []UserID{uid}, u.UserIDs())
}

func TestMembers_HasUser(t *testing.T) {
	uid := NewUserID()
	u := &Members{users: map[UserID]Member{uid: {Role: RoleOwner}}}
	assert.True(t, u.HasUser(uid))
	assert.False(t, u.HasUser(NewUserID()))
}

func TestMembers_UsersByRole(t *testing.T) {
	uid := NewUserID()
	uid2 := NewUserID()
	uid3 := NewUserID()
	u := &Members{users: map[UserID]Member{uid: {Role: RoleOwner}, uid2: {Role: RoleOwner}, uid3: {Role: RoleReader}}}
	assert.Equal(t, []UserID{uid2, uid}, u.UsersByRole(RoleOwner))
	assert.Equal(t, []UserID{uid3}, u.UsersByRole(RoleReader))
	assert.Equal(t, []UserID{}, u.UsersByRole(RoleWriter))
}

func TestMembers_Integrations(t *testing.T) {
	iid := NewIntegrationID()
	u := &Members{integrations: map[IntegrationID]Member{iid: {Role: RoleOwner}}}
	assert.Equal(t, map[IntegrationID]Member{iid: {Role: RoleOwner}}, u.Integrations())
	assert.Equal(t, []IntegrationID{iid}, u.IntegrationIDs())
}

func TestMembers_HasIntegration(t *testing.T) {
	iid1 := NewIntegrationID()
	iid2 := NewIntegrationID()
	u := &Members{integrations: map[IntegrationID]Member{iid1: {Role: RoleOwner}}}
	assert.True(t, u.HasIntegration(iid1))
	assert.False(t, u.HasIntegration(iid2))
}

func TestMembers_User(t *testing.T) {
	uid := NewUserID()
	m := &Members{users: map[UserID]Member{uid: {Role: RoleOwner}}}
	assert.Equal(t, &Member{Role: RoleOwner}, m.User(uid))
	assert.Nil(t, m.User(NewUserID()))
}

func TestMembers_Integration(t *testing.T) {
	iid := NewIntegrationID()
	m := &Members{integrations: map[IntegrationID]Member{iid: {Role: RoleOwner}}}
	assert.Equal(t, &Member{Role: RoleOwner}, m.Integration(iid))
	assert.Nil(t, m.Integration(NewIntegrationID()))
}

func TestMembers_Count(t *testing.T) {
	assert.Equal(t, 0, (&Members{}).Count())
	assert.True(t, (&Members{}).IsEmpty())

	uid := NewUserID()
	assert.Equal(t, 1, (&Members{users: map[UserID]Member{uid: {Role: RoleOwner}}}).Count())
	assert.False(t, (&Members{users: map[UserID]Member{uid: {Role: RoleOwner}}}).IsEmpty())
}

func TestMembers_Fixed(t *testing.T) {
	assert.True(t, (&Members{fixed: true}).Fixed())
	assert.False(t, (&Members{fixed: false}).Fixed())
}

func TestMembers_IsOnlyOwner(t *testing.T) {
	uid := NewUserID()
	assert.True(t, (&Members{
		users: map[UserID]Member{uid: {Role: RoleOwner}},
	}).IsOnlyOwner(uid))
}

func TestMembers_IsOwnerOrMaintainer(t *testing.T) {
	uid := NewUserID()
	assert.True(t, (&Members{
		users: map[UserID]Member{uid: {Role: RoleMaintainer}},
	}).IsOwnerOrMaintainer(uid))

	assert.True(t, (&Members{
		users: map[UserID]Member{uid: {Role: RoleOwner}},
	}).IsOwnerOrMaintainer(uid))

	assert.False(t, (&Members{
		users: map[UserID]Member{uid: {Role: RoleReader}},
	}).IsOwnerOrMaintainer(uid))
}

func TestMembers_Join(t *testing.T) {
	uid := NewUserID()
	uid2 := NewUserID()

	u := user.New().ID(uid).Name("test").Email("test@example.com").MustBuild().WithHost("reearth")

	// ok
	m := &Members{}
	assert.NoError(t, m.Join(u, RoleOwner, uid2))
	assert.Equal(t, map[UserID]Member{
		uid: {Role: RoleOwner, InvitedBy: uid2, Host: "reearth"},
	}, m.users)

	// fixed
	m = &Members{fixed: true}
	assert.Equal(t, ErrCannotModifyPersonalWorkspace, m.Join(u, RoleOwner, uid2))
	assert.Nil(t, m.users)

	// already joined
	m = &Members{users: map[UserID]Member{uid: {Role: RoleOwner}}}
	assert.Equal(t, ErrUserAlreadyJoined, m.Join(u, RoleOwner, uid2))
	assert.Equal(t, map[UserID]Member{uid: {Role: RoleOwner}}, m.users)
}

func TestMembers_LeaveUser(t *testing.T) {
	uid := NewUserID()
	uid2 := NewUserID()

	// ok
	m := &Members{users: map[UserID]Member{uid: {Role: RoleOwner}, uid2: {Role: RoleOwner}}}
	assert.NoError(t, m.Leave(uid2))
	assert.Equal(t, map[UserID]Member{
		uid: {Role: RoleOwner},
	}, m.users)

	// fixed
	m = &Members{fixed: true}
	assert.Equal(t, ErrCannotModifyPersonalWorkspace, m.Leave(uid))
	assert.Nil(t, m.users)

	// no user
	m = &Members{users: map[UserID]Member{uid: {Role: RoleOwner}}}
	assert.Equal(t, ErrTargetUserNotInTheWorkspace, m.Leave(uid2))
	assert.Equal(t, map[UserID]Member{uid: {Role: RoleOwner}}, m.users)
}

func TestMembers_UpdateUserRole(t *testing.T) {
	uid := NewUserID()
	uid2 := NewUserID()

	// ok
	m := &Members{users: map[UserID]Member{uid: {Role: RoleOwner}, uid2: {Role: RoleOwner}}}
	assert.NoError(t, m.UpdateUserRole(uid2, RoleReader))
	assert.Equal(t, map[UserID]Member{
		uid:  {Role: RoleOwner},
		uid2: {Role: RoleReader},
	}, m.users)

	// fixed
	m = &Members{fixed: true}
	assert.Equal(t, ErrCannotModifyPersonalWorkspace, m.UpdateUserRole(uid, RoleOwner))
	assert.Nil(t, m.users)

	// no user
	m = &Members{users: map[UserID]Member{uid: {Role: RoleOwner}}}
	assert.Equal(t, ErrTargetUserNotInTheWorkspace, m.UpdateUserRole(uid2, RoleOwner))
	assert.Equal(t, map[UserID]Member{uid: {Role: RoleOwner}}, m.users)
}

func TestMembers_AddIntegration(t *testing.T) {
	iid := NewIntegrationID()
	uid := NewUserID()

	// ok
	m := &Members{}
	assert.NoError(t, m.AddIntegration(iid, RoleOwner, uid))
	assert.Equal(t, map[IntegrationID]Member{
		iid: {Role: RoleOwner, InvitedBy: uid},
	}, m.integrations)

	// already added
	m = &Members{integrations: map[IntegrationID]Member{iid: {Role: RoleOwner}}}
	assert.Equal(t, ErrUserAlreadyJoined, m.AddIntegration(iid, RoleOwner, uid))
	assert.Equal(t, map[IntegrationID]Member{iid: {Role: RoleOwner}}, m.integrations)
}

func TestMembers_DeleteIntegration(t *testing.T) {
	uid := NewIntegrationID()
	uid2 := NewIntegrationID()

	// ok
	m := &Members{integrations: map[IntegrationID]Member{uid: {Role: RoleOwner}, uid2: {Role: RoleOwner}}}
	assert.NoError(t, m.DeleteIntegration(uid2))
	assert.Equal(t, map[IntegrationID]Member{
		uid: {Role: RoleOwner},
	}, m.integrations)

	// no integrations
	m = &Members{integrations: map[IntegrationID]Member{uid: {Role: RoleOwner}}}
	assert.Equal(t, ErrTargetUserNotInTheWorkspace, m.DeleteIntegration(uid2))
	assert.Equal(t, map[IntegrationID]Member{uid: {Role: RoleOwner}}, m.integrations)
}

func TestMembers_UpdateIntegrationRole(t *testing.T) {
	uid := NewIntegrationID()
	uid2 := NewIntegrationID()

	// ok
	m := &Members{integrations: map[IntegrationID]Member{uid: {Role: RoleOwner}, uid2: {Role: RoleOwner}}}
	assert.NoError(t, m.UpdateIntegrationRole(uid2, RoleReader))
	assert.Equal(t, map[IntegrationID]Member{
		uid:  {Role: RoleOwner},
		uid2: {Role: RoleReader},
	}, m.integrations)

	// no user
	m = &Members{integrations: map[IntegrationID]Member{uid: {Role: RoleOwner}}}
	assert.Equal(t, ErrTargetUserNotInTheWorkspace, m.UpdateIntegrationRole(uid2, RoleOwner))
	assert.Equal(t, map[IntegrationID]Member{uid: {Role: RoleOwner}}, m.integrations)
}

func TestMembers_DeleteIntegrations(t *testing.T) {
	uid1 := NewIntegrationID()
	uid2 := NewIntegrationID()
	uid3 := NewIntegrationID() // not in map
	uid4 := NewIntegrationID()

	tests := []struct {
		name          string
		initial       map[IntegrationID]Member
		input         []IntegrationID
		wantErr       error
		wantErrText   string
		expectedState map[IntegrationID]Member
	}{
		{
			name: "delete multiple existing integrations",
			initial: map[IntegrationID]Member{
				uid1: {Role: RoleOwner},
				uid2: {Role: RoleMaintainer},
				uid4: {Role: RoleReader},
			},
			input:   []IntegrationID{uid1, uid2},
			wantErr: nil,
			expectedState: map[IntegrationID]Member{
				uid4: {Role: RoleReader},
			},
		},
		{
			name: "empty input slice",
			initial: map[IntegrationID]Member{
				uid1: {Role: RoleOwner},
			},
			input:         []IntegrationID{},
			wantErr:       ErrNoSpecifiedUsers,
			expectedState: map[IntegrationID]Member{uid1: {Role: RoleOwner}},
		},
		{
			name: "some IDs not found",
			initial: map[IntegrationID]Member{
				uid1: {Role: RoleOwner},
				uid2: {Role: RoleReader},
			},
			input:         []IntegrationID{uid1, uid3}, // uid3 doesn't exist
			wantErr:       ErrTargetUserNotInTheWorkspace,
			wantErrText:   uid3.String(), // make sure it's included in error string
			expectedState: map[IntegrationID]Member{uid1: {Role: RoleOwner}, uid2: {Role: RoleReader}},
		},
		{
			name: "all IDs missing",
			initial: map[IntegrationID]Member{
				uid1: {Role: RoleOwner},
			},
			input:         []IntegrationID{uid3},
			wantErr:       ErrTargetUserNotInTheWorkspace,
			wantErrText:   uid3.String(),
			expectedState: map[IntegrationID]Member{uid1: {Role: RoleOwner}},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := &Members{
				integrations: copyIntegrationMap(tt.initial),
			}
			err := m.DeleteIntegrations(tt.input)

			if tt.wantErr != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				if tt.wantErrText != "" {
					assert.Contains(t, err.Error(), tt.wantErrText)
				}
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedState, m.integrations)
		})
	}
}

func copyIntegrationMap(src map[IntegrationID]Member) map[IntegrationID]Member {
	dst := make(map[IntegrationID]Member, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
