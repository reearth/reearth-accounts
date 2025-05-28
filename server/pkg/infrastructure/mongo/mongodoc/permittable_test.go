package mongodoc

import (
	"io"
	"testing"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

func TestPermittableDocument_NewPermittableConsumer(t *testing.T) {
	pid := id.NewPermittableID()
	p, _ := permittable.New().
		ID(pid).
		UserID(user.NewID()).
		RoleIDs([]id.RoleID{id.NewRoleID()}).
		Build()
	doc1, _ := NewPermittable(*p)
	p1 := lo.Must(bson.Marshal(doc1))

	tests := []struct {
		name    string
		arg     bson.Raw
		wantErr bool
		wantEOF bool
		result  []*permittable.Permittable
	}{
		{
			name:    "consume permittable item",
			arg:     p1,
			wantErr: false,
			wantEOF: false,
			result:  []*permittable.Permittable{p},
		},
		{
			name:    "fail: unmarshal error",
			arg:     []byte{},
			wantErr: true,
			wantEOF: false,
			result:  nil,
		},
		{
			name:    "nil",
			arg:     nil,
			wantErr: false,
			wantEOF: true,
			result:  nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := NewPermittableConsumer()
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

func TestPermittableDocument_NewPermittable(t *testing.T) {
	pid := id.NewPermittableID()
	uid := user.NewID()
	rid1 := id.NewRoleID()
	rid2 := id.NewRoleID()
	p, _ := permittable.New().
		ID(pid).
		UserID(uid).
		RoleIDs([]id.RoleID{rid1, rid2}).
		Build()
	type args struct {
		p permittable.Permittable
	}

	tests := []struct {
		name  string
		args  args
		want  *PermittableDocument
		want1 string
	}{
		{
			name: "New permittable",
			args: args{
				p: *p,
			},
			want: &PermittableDocument{
				ID:      pid.String(),
				UserID:  uid.String(),
				RoleIDs: []string{rid1.String(), rid2.String()},
			},
			want1: pid.String(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, got1 := NewPermittable(tc.args.p)
			assert.Equal(t, tc.want1, got1)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestPermittableDocument_Model(t *testing.T) {
	pid := id.NewPermittableID()
	uid := user.NewID()
	rid1 := id.NewRoleID()
	rid2 := id.NewRoleID()

	tests := []struct {
		name            string
		doc             *PermittableDocument
		expectedID      id.PermittableID
		expectedUserID  user.ID
		expectedRoleIDs []id.RoleID
		expectErr       bool
	}{
		{
			name: "valid permittable document",
			doc: &PermittableDocument{
				ID:      pid.String(),
				UserID:  uid.String(),
				RoleIDs: []string{rid1.String(), rid2.String()},
			},
			expectedID:     pid,
			expectedUserID: uid,
			expectedRoleIDs: []id.RoleID{
				rid1,
				rid2,
			},
			expectErr: false,
		},
		{
			name:      "nil document",
			doc:       nil,
			expectErr: false,
		},
		{
			name: "invalid permittable ID",
			doc: &PermittableDocument{
				ID:      "invalid-id",
				UserID:  uid.String(),
				RoleIDs: []string{rid1.String(), rid2.String()},
			},
			expectErr: true,
		},
		{
			name: "invalid user ID",
			doc: &PermittableDocument{
				ID:      pid.String(),
				UserID:  "invalid-id",
				RoleIDs: []string{rid1.String(), rid2.String()},
			},
			expectErr: true,
		},
		{
			name: "invalid role ID",
			doc: &PermittableDocument{
				ID:      pid.String(),
				UserID:  uid.String(),
				RoleIDs: []string{"invalid-id"},
			},
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, err := tc.doc.Model()

			if tc.expectErr {
				assert.Error(t, err)
				assert.Nil(t, r)
			} else {
				assert.NoError(t, err)
				if tc.doc != nil {
					assert.Equal(t, tc.expectedID, r.ID())
					assert.Equal(t, tc.expectedUserID, r.UserID())
					assert.Equal(t, tc.expectedRoleIDs, r.RoleIDs())
				} else {
					assert.Nil(t, r)
				}
			}
		})
	}
}
