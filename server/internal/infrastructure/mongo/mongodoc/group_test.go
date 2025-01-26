package mongodoc

import (
	"io"
	"testing"

	"github.com/eukarya-inc/reearth-dashboard/pkg/group"
	"github.com/eukarya-inc/reearth-dashboard/pkg/id"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

func TestGroupDocument_NewGroupConsumer(t *testing.T) {
	gid := id.NewGroupID()
	g, _ := group.New().
		ID(gid).
		Name("hoge").
		Build()
	doc1, _ := NewGroup(*g)
	g1 := lo.Must(bson.Marshal(doc1))

	tests := []struct {
		name    string
		arg     bson.Raw
		wantErr bool
		wantEOF bool
		result  []*group.Group
	}{
		{
			name:    "nil",
			arg:     nil,
			wantErr: false,
			wantEOF: true,
			result:  nil,
		},
		{
			name:    "consume group item",
			arg:     g1,
			wantErr: false,
			wantEOF: false,
			result:  []*group.Group{g},
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
			c := NewGroupConsumer()
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

func TestGroupDocument_NewGroup(t *testing.T) {
	gid := id.NewGroupID()
	g, _ := group.New().
		ID(gid).
		Name("hoge").
		Build()
	type args struct {
		g group.Group
	}

	tests := []struct {
		name  string
		args  args
		want  *GroupDocument
		want1 string
	}{
		{
			name: "New group",
			args: args{
				g: *g,
			},
			want: &GroupDocument{
				ID:   gid.String(),
				Name: "hoge",
			},
			want1: gid.String(),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, got1 := NewGroup(tc.args.g)
			assert.Equal(t, tc.want1, got1)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestGroupDocument_Model(t *testing.T) {
	gid := id.NewGroupID()

	tests := []struct {
		name         string
		doc          *GroupDocument
		expectedID   id.GroupID
		expectedName string
		expectErr    bool
	}{
		{
			name:         "valid group document",
			doc:          &GroupDocument{ID: gid.String(), Name: "admin"},
			expectedID:   gid,
			expectedName: "admin",
			expectErr:    false,
		},
		{
			name:      "nil document",
			doc:       nil,
			expectErr: false,
		},
		{
			name:      "invalid group ID",
			doc:       &GroupDocument{ID: "invalid-id", Name: "user"},
			expectErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g, err := tc.doc.Model()

			if tc.expectErr {
				assert.Error(t, err)
				assert.Nil(t, g)
			} else {
				assert.NoError(t, err)
				if tc.doc != nil {
					assert.Equal(t, tc.expectedID, g.ID())
					assert.Equal(t, tc.expectedName, g.Name())
				} else {
					assert.Nil(t, g)
				}
			}
		})
	}
}
