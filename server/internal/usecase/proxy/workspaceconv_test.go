package proxy

import (
	"errors"
	"reflect"
	"testing"

	"github.com/reearth/reearth-accounts/pkg/id"
	"github.com/reearth/reearth-accounts/pkg/workspace"
)

func TestWorkspaceByIDsResponseTo(t *testing.T) {
	wid := id.NewWorkspaceID()
	uid := id.NewUserID()
	iid := id.NewIntegrationID()
	um := &FragmentWorkspaceMembersWorkspaceUserMember{
		Typename: "WorkspaceUserMember",
		UserId:   uid.String(),
		Role:     RoleOwner,
	}
	im := &FragmentWorkspaceMembersWorkspaceIntegrationMember{
		Typename:      "WorkspaceIntegrationMember",
		IntegrationId: iid.String(),
		Role:          RoleReader,
		InvitedById:   uid.String(),
	}

	w := &WorkspaceByIDsNodesWorkspace{
		FragmentWorkspace: FragmentWorkspace{
			Id:       wid.String(),
			Name:     "name",
			Personal: true,
			Members: []FragmentWorkspaceMembersWorkspaceMember{
				um, im,
			},
		},
		Typename: "Workspace",
	}
	owner := workspace.Member{
		Role: workspace.RoleOwner,
	}
	reader := workspace.Member{
		Role:      workspace.RoleReader,
		InvitedBy: uid,
	}

	ws := workspace.New().ID(wid).Name("name").
		Personal(true).
		Members(map[id.UserID]workspace.Member{uid: owner}).
		Integrations(map[id.IntegrationID]workspace.Member{iid: reader}).
		MustBuild()

	type args struct {
		r   *WorkspaceByIDsResponse
		err error
	}
	tests := []struct {
		name    string
		args    args
		want    []*workspace.Workspace
		wantErr bool
	}{
		{
			name: "ok",
			args: args{
				&WorkspaceByIDsResponse{
					[]WorkspaceByIDsNodesNode{
						w,
					},
				},
				nil,
			},
			want: []*workspace.Workspace{
				ws,
			},
			wantErr: false,
		},
		{
			name: "NG",
			args: args{
				&WorkspaceByIDsResponse{
					[]WorkspaceByIDsNodesNode{
						w,
					},
				},
				errors.New("test"),
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := WorkspaceByIDsResponseTo(tt.args.r, tt.args.err)
			if (err != nil) != tt.wantErr {
				t.Errorf("WorkspaceByIDsResponseTo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("WorkspaceByIDsResponseTo() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestWorkspaceByIDsNodeTo(t *testing.T) {
	wid := id.NewWorkspaceID()
	uid := id.NewUserID()
	iid := id.NewIntegrationID()
	um := &FragmentWorkspaceMembersWorkspaceUserMember{
		Typename: "WorkspaceUserMember",
		UserId:   uid.String(),
		Role:     RoleOwner,
	}
	im := &FragmentWorkspaceMembersWorkspaceIntegrationMember{
		Typename:      "WorkspaceIntegrationMember",
		IntegrationId: iid.String(),
		Role:          RoleReader,
		InvitedById:   uid.String(),
	}

	w := &WorkspaceByIDsNodesWorkspace{
		FragmentWorkspace: FragmentWorkspace{
			Id:       wid.String(),
			Name:     "name",
			Personal: true,
			Members: []FragmentWorkspaceMembersWorkspaceMember{
				um, im,
			},
		},
		Typename: "Workspace",
	}
	owner := workspace.Member{
		Role: workspace.RoleOwner,
	}
	reader := workspace.Member{
		Role:      workspace.RoleReader,
		InvitedBy: uid,
	}

	ws := workspace.New().ID(wid).Name("name").
		Personal(true).
		Members(map[id.UserID]workspace.Member{uid: owner}).
		Integrations(map[id.IntegrationID]workspace.Member{iid: reader}).
		MustBuild()

	type args struct {
		r WorkspaceByIDsNodesNode
	}
	tests := []struct {
		name    string
		args    args
		want    *workspace.Workspace
		wantErr bool
	}{
		{
			name: "ok",
			args: args{
				r: w,
			},
			want:    ws,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := WorkspaceByIDsNodeTo(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("WorkspaceByIDsNodeTo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("WorkspaceByIDsNodeTo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToWorkspaces(t *testing.T) {
	wid := id.NewWorkspaceID()
	uid := id.NewUserID()
	iid := id.NewIntegrationID()
	um := &FragmentWorkspaceMembersWorkspaceUserMember{
		Typename: "WorkspaceUserMember",
		UserId:   uid.String(),
		Role:     RoleOwner,
	}
	im := &FragmentWorkspaceMembersWorkspaceIntegrationMember{
		Typename:      "WorkspaceIntegrationMember",
		IntegrationId: iid.String(),
		Role:          RoleReader,
		InvitedById:   uid.String(),
	}

	w := FragmentWorkspace{
		Id:       wid.String(),
		Name:     "name",
		Personal: true,
		Members: []FragmentWorkspaceMembersWorkspaceMember{
			um, im,
		},
	}
	owner := workspace.Member{
		Role: workspace.RoleOwner,
	}
	reader := workspace.Member{
		Role:      workspace.RoleReader,
		InvitedBy: uid,
	}

	ws := workspace.New().ID(wid).Name("name").
		Personal(true).
		Members(map[id.UserID]workspace.Member{uid: owner}).
		Integrations(map[id.IntegrationID]workspace.Member{iid: reader}).
		MustBuild()

	type args struct {
		r []FragmentWorkspace
	}
	tests := []struct {
		name    string
		args    args
		want    []*workspace.Workspace
		wantErr bool
	}{
		{
			name: "ok",
			args: args{
				r: []FragmentWorkspace{w},
			},
			want:    []*workspace.Workspace{ws},
			wantErr: false,
		}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToWorkspaces(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToWorkspaces() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ToWorkspaces() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToWorkspace(t *testing.T) {
	wid := id.NewWorkspaceID()
	uid := id.NewUserID()
	iid := id.NewIntegrationID()
	um := &FragmentWorkspaceMembersWorkspaceUserMember{
		Typename: "WorkspaceUserMember",
		UserId:   uid.String(),
		Role:     RoleOwner,
	}
	im := &FragmentWorkspaceMembersWorkspaceIntegrationMember{
		Typename:      "WorkspaceIntegrationMember",
		IntegrationId: iid.String(),
		Role:          RoleReader,
		InvitedById:   uid.String(),
	}

	w := FragmentWorkspace{
		Id:       wid.String(),
		Name:     "name",
		Personal: true,
		Members: []FragmentWorkspaceMembersWorkspaceMember{
			um, im,
		},
	}
	owner := workspace.Member{
		Role: workspace.RoleOwner,
	}
	reader := workspace.Member{
		Role:      workspace.RoleReader,
		InvitedBy: uid,
	}

	ws := workspace.New().ID(wid).Name("name").
		Personal(true).
		Members(map[id.UserID]workspace.Member{uid: owner}).
		Integrations(map[id.IntegrationID]workspace.Member{iid: reader}).
		MustBuild()

	type args struct {
		r FragmentWorkspace
	}
	tests := []struct {
		name    string
		args    args
		want    *workspace.Workspace
		wantErr bool
	}{
		{
			name: "ok",
			args: args{
				r: w,
			},
			want:    ws,
			wantErr: false,
		}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToWorkspace(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToWorkspace() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ToWorkspace() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToRole(t *testing.T) {
	type args struct {
		r Role
	}
	tests := []struct {
		name string
		args args
		want workspace.Role
	}{
		{
			name: "ok maintainer",
			args: args{
				r: RoleMaintainer,
			},
			want: workspace.RoleMaintainer,
		},
		{
			name: "ok reader",
			args: args{
				r: RoleReader,
			},
			want: workspace.RoleReader,
		},
		{
			name: "ok owner",
			args: args{
				r: RoleOwner,
			},
			want: workspace.RoleOwner,
		},
		{
			name: "ok writer",
			args: args{
				r: RoleWriter,
			},
			want: workspace.RoleWriter,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToRole(tt.args.r); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ToRole() = %v, want %v", got, tt.want)
			}
		})
	}
}
