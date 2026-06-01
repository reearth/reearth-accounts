package interactor

import (
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
)

func TestUc_CheckPermission(t *testing.T) {
	tid := id.NewWorkspaceID()

	tests := []struct {
		name               string
		op                 *workspace.Operator
		readableWorkspaces id.WorkspaceIDList
		writableWorkspaces id.WorkspaceIDList
		wantErr            bool
	}{
		{
			name:    "nil operator",
			wantErr: false,
		},
		{
			name:               "nil operator 2",
			readableWorkspaces: id.WorkspaceIDList{id.NewWorkspaceID()},
			wantErr:            false,
		},
		{
			name:               "can read a workspace",
			readableWorkspaces: id.WorkspaceIDList{tid},
			op: &workspace.Operator{
				ReadableWorkspaces: id.WorkspaceIDList{tid},
			},
			wantErr: false,
		},
		{
			name:               "cannot read a workspace",
			readableWorkspaces: id.WorkspaceIDList{id.NewWorkspaceID()},
			op: &workspace.Operator{
				ReadableWorkspaces: id.WorkspaceIDList{},
			},
			wantErr: true,
		},
		{
			name:               "can write a workspace",
			writableWorkspaces: id.WorkspaceIDList{tid},
			op: &workspace.Operator{
				WritableWorkspaces: id.WorkspaceIDList{tid},
			},
			wantErr: false,
		},
		{
			name:               "cannot write a workspace",
			writableWorkspaces: id.WorkspaceIDList{tid},
			op: &workspace.Operator{
				WritableWorkspaces: id.WorkspaceIDList{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			e := &uc{
				readableWorkspaces: tt.readableWorkspaces,
				writableWorkspaces: tt.writableWorkspaces,
			}
			got := e.CheckPermission(tt.op)
			if tt.wantErr {
				assert.Equal(t, interfaces.ErrOperationDenied, got)
			} else {
				assert.Nil(t, got)
			}
		})
	}
}

func TestUc(t *testing.T) {
	workspaces := id.WorkspaceIDList{id.NewWorkspaceID(), id.NewWorkspaceID(), id.NewWorkspaceID()}
	assert.Equal(t, &uc{}, Usecase())
	assert.Equal(t, &uc{readableWorkspaces: workspaces}, (&uc{}).WithReadableWorkspaces(workspaces...))
	assert.Equal(t, &uc{writableWorkspaces: workspaces}, (&uc{}).WithWritableWorkspaces(workspaces...))
}
