package role

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuilder_New(t *testing.T) {
	var tb = New()
	assert.NotNil(t, tb)
}

func TestBuilder_Build(t *testing.T) {
	rid := NewID()

	type args struct {
		id   ID
		name string
	}

	tests := []struct {
		Name     string
		Args     args
		Expected *Role
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
			Name: "fail empty name",
			Args: args{
				id:   rid,
				name: "",
			},
			Err: ErrEmptyName,
		},
		{
			Name: "success build new role",
			Args: args{
				id:   rid,
				name: "test",
			},
			Expected: &Role{
				id:   rid,
				name: "test",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()
			res, err := New().
				ID(tt.Args.id).
				Name(tt.Args.name).
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
	rid := NewID()

	type args struct {
		id   ID
		name string
	}

	tests := []struct {
		Name     string
		Args     args
		Expected *Role
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
			Name: "fail empty name",
			Args: args{
				id:   rid,
				name: "",
			},
			Err: ErrEmptyName,
		},
		{
			Name: "success build new role",
			Args: args{
				id:   rid,
				name: "test",
			},
			Expected: &Role{
				id:   rid,
				name: "test",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			build := func() *Role {
				t.Helper()
				return New().
					ID(tt.Args.id).
					Name(tt.Args.name).
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
	res := tb.ID(NewID()).Name("hoge").MustBuild()
	assert.NotNil(t, res.ID())
}

func TestBuilder_NewID(t *testing.T) {
	var tb = New()
	res := tb.NewID().Name("hoge").MustBuild()
	assert.NotNil(t, res.ID())
}

func TestBuilder_Name(t *testing.T) {
	var tb = New().NewID()
	res := tb.Name("hoge").MustBuild()
	assert.Equal(t, "hoge", res.Name())
}
