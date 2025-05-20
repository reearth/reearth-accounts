package permittable

import (
	"testing"

	"github.com/reearth/reearth-accounts/pkg/id"
	"github.com/reearth/reearth-accounts/pkg/role"
	"github.com/reearth/reearth-accounts/pkg/user"
	"github.com/stretchr/testify/assert"
)

func TestBuilder_New(t *testing.T) {
	var tb = New()
	assert.NotNil(t, tb)
}

func TestBuilder_Build(t *testing.T) {
	pid := NewID()
	uid := user.NewID()

	type args struct {
		id     ID
		userId user.ID
	}

	tests := []struct {
		Name     string
		Args     args
		Expected *Permittable
		Err      error
	}{
		{
			Name: "fail nil id",
			Args: args{
				id: ID{},
			},
			Err: ErrInvalidID,
		},
		{
			Name: "fail nil user id",
			Args: args{
				id:     pid,
				userId: user.ID{},
			},
			Err: ErrInvalidID,
		},
		{
			Name: "success build new permittable",
			Args: args{
				id:     pid,
				userId: uid,
			},
			Expected: &Permittable{
				id:     pid,
				userID: uid,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()
			res, err := New().
				ID(tt.Args.id).
				UserID(tt.Args.userId).
				Build()

			if tt.Err == nil {
				assert.Equal(t, tt.Expected, res)
			} else {
				assert.Equal(t, tt.Err, err)
			}
		})
	}
}

func TestBuilder_MustBuild(t *testing.T) {
	pid := NewID()
	uid := user.NewID()

	type args struct {
		id     ID
		userId user.ID
	}

	tests := []struct {
		Name     string
		Args     args
		Expected *Permittable
		Err      error
	}{
		{
			Name: "fail nil id",
			Args: args{
				id: ID{},
			},
			Err: ErrInvalidID,
		},
		{
			Name: "fail nil user id",
			Args: args{
				id:     pid,
				userId: user.ID{},
			},
			Err: ErrInvalidID,
		},
		{
			Name: "success build new permittable",
			Args: args{
				id:     pid,
				userId: uid,
			},
			Expected: &Permittable{
				id:     pid,
				userID: uid,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			build := func() *Permittable {
				t.Helper()
				return New().
					ID(tt.Args.id).
					UserID(tt.Args.userId).
					MustBuild()
			}

			if tt.Err != nil {
				assert.PanicsWithValue(t, tt.Err, func() { _ = build() })
			} else {
				assert.Equal(t, tt.Expected, build())
			}
		})
	}
}

func TestBuilder_ID(t *testing.T) {
	var tb = New()
	res := tb.ID(NewID()).UserID(user.NewID()).MustBuild()
	assert.NotNil(t, res.ID())
}

func TestBuilder_NewID(t *testing.T) {
	var tb = New()
	res := tb.NewID().UserID(user.NewID()).MustBuild()
	assert.NotNil(t, res.ID())
}

func TestBuilder_UserID(t *testing.T) {
	var tb = New().NewID()
	res := tb.UserID(user.NewID()).MustBuild()
	assert.NotNil(t, res.UserID())
}

func TestBuilder_RoleIDs(t *testing.T) {
	var tb = New().NewID().UserID(user.NewID())
	res := tb.RoleIDs([]id.RoleID{role.NewID()}).MustBuild()
	assert.NotNil(t, res.RoleIDs())
}
