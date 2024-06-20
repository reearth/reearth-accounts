package mongodoc

import (
	"io"
	"testing"

	"github.com/reearth/reearth-account/pkg/id"
	"github.com/reearth/reearth-account/pkg/role"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

func TestRoleDocument_NewRoleConsumer(t *testing.T) {
	rid := id.NewRoleID()
	r, _ := role.New().
		ID(rid).
		Name("hoge").
		Build()
	doc1, _ := NewRole(*r)
	r1 := lo.Must(bson.Marshal(doc1))

	tests := []struct {
		name    string
		arg     bson.Raw
		wantErr bool
		wantEOF bool
		result  []*role.Role
	}{
		{
			name:    "nil",
			arg:     nil,
			wantErr: false,
			wantEOF: true,
			result:  nil,
		},
		{
			name:    "consume role item",
			arg:     r1,
			wantErr: false,
			wantEOF: false,
			result:  []*role.Role{r},
		},
		{
			name:    "fail: unmarshal error",
			arg:     []byte{},
			wantErr: true,
			wantEOF: false,
			result:  nil,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := NewRoleConsumer()
			err := c.Consume(tc.arg)
			if tc.wantEOF {
				assert.Equal(t, io.EOF, err)
			} else if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.result, c.Result)
		})
	}
}

func TestRoleDocument_NewRole(t *testing.T) {
	rid := id.NewRoleID()
	r, _ := role.New().
		ID(rid).
		Name("hoge").
		Build()
	type args struct {
		r role.Role
	}

	tests := []struct {
		name  string
		args  args
		want  *RoleDocument
		want1 string
	}{
		{
			name: "New role",
			args: args{
				r: *r,
			},
			want: &RoleDocument{
				ID:   rid.String(),
				Name: "hoge",
			},
			want1: rid.String(),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, got1 := NewRole(tc.args.r)
			assert.Equal(t, tc.want1, got1)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRoleDocument_Model(t *testing.T) {
	rid := id.NewRoleID()

	tests := []struct {
		name         string
		doc          *RoleDocument
		expectedID   id.RoleID
		expectedName string
		expectErr    bool
	}{
		{
			name:         "valid role document",
			doc:          &RoleDocument{ID: rid.String(), Name: "admin"},
			expectedID:   rid,
			expectedName: "admin",
			expectErr:    false,
		},
		{
			name:      "nil document",
			doc:       nil,
			expectErr: false,
		},
		{
			name:      "invalid role ID",
			doc:       &RoleDocument{ID: "invalid-id", Name: "user"},
			expectErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			r, err := tc.doc.Model()

			if tc.expectErr {
				assert.Error(t, err)
				assert.Nil(t, r)
			} else {
				assert.NoError(t, err)
				if tc.doc != nil {
					assert.Equal(t, tc.expectedID, r.ID())
					assert.Equal(t, tc.expectedName, r.Name())
				} else {
					assert.Nil(t, r)
				}
			}
		})
	}
}
