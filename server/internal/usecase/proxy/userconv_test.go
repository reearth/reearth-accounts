package proxy

import (
	"errors"
	"reflect"
	"testing"

	"github.com/reearth/reearth-accounts/pkg/id"
	"github.com/reearth/reearth-accounts/pkg/user"
	"golang.org/x/text/language"
)

func TestUserByIDsResponseTo(t *testing.T) {
	uid := id.NewUserID()
	ws := id.NewWorkspaceID()
	u := &UserByIDsNodesUser{
		Id:        uid.String(),
		Name:      "name",
		Email:     "email@example.com",
		Workspace: ws.String(),
		Typename:  "User",
	}
	us := user.New().ID(uid).Name("name").
		Email("email@example.com").
		Workspace(ws).
		MustBuild()

	type args struct {
		r   *UserByIDsResponse
		err error
	}
	tests := []struct {
		name    string
		args    args
		want    []*user.User
		wantErr bool
	}{
		{
			name: "ok",
			args: args{
				&UserByIDsResponse{
					[]UserByIDsNodesNode{
						u,
					},
				},
				nil,
			},
			want: []*user.User{
				us,
			},
			wantErr: false,
		},
		{
			name: "error",
			args: args{
				nil,
				errors.New("test"),
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UserByIDsResponseTo(tt.args.r, tt.args.err)
			if (err != nil) != tt.wantErr {
				t.Errorf("UserByIDsResponseTo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UserByIDsResponseTo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSimpleUserByIDsResponseTo(t *testing.T) {
	uid := id.NewUserID()
	u := &UserByIDsNodesUser{
		Id:       uid.String(),
		Name:     "name",
		Email:    "email",
		Typename: "User",
	}

	type args struct {
		r   *UserByIDsResponse
		err error
	}
	tests := []struct {
		name    string
		args    args
		want    []*user.Simple
		wantErr bool
	}{
		{
			name: "ok",
			args: args{
				&UserByIDsResponse{
					[]UserByIDsNodesNode{
						u,
					},
				},
				nil,
			},
			want: []*user.Simple{
				{
					ID:    uid,
					Name:  "name",
					Email: "email",
				},
			},
			wantErr: false,
		},
		{
			name: "error",
			args: args{
				nil,
				errors.New("test"),
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SimpleUserByIDsResponseTo(tt.args.r, tt.args.err)
			if (err != nil) != tt.wantErr {
				t.Errorf("SimpleUserByIDsResponseTo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SimpleUserByIDsResponseTo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMeToUser(t *testing.T) {
	uid := id.NewUserID()
	wid := id.NewWorkspaceID()
	type args struct {
		me FragmentMe
	}
	tests := []struct {
		name    string
		args    args
		want    *user.User
		wantErr bool
	}{
		{
			name: "ok",
			args: args{
				FragmentMe{
					Id:            uid.String(),
					Name:          "name",
					Email:         "test@exmple.com",
					Lang:          "ja",
					Theme:         "dark",
					MyWorkspaceId: wid.String(),
					Auths:         []string{"foo|bar"},
				},
			},
			want: user.New().ID(uid).Name("name").
				Email("test@exmple.com").LangFrom("ja").Theme(user.ThemeDark).
				Workspace(wid).Auths([]user.Auth{{Provider: "foo", Sub: "foo|bar"}}).MustBuild(),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MeToUser(tt.args.me)
			if (err != nil) != tt.wantErr {
				t.Errorf("MeToUser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MeToUser() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFragmentToUser(t *testing.T) {
	uid := id.NewUserID()
	ws := id.NewWorkspaceID()
	u := FragmentUser{
		Id:        uid.String(),
		Name:      "name",
		Email:     "email@example.com",
		Workspace: ws.String(),
		Lang:      "ja",
		Theme:     "DARK",
		Auths:     []string{"sub"},
	}
	auth := user.AuthFrom("sub")
	us := user.New().ID(uid).Name("name").
		Email("email@example.com").
		Lang(language.Japanese).
		Theme(user.ThemeDark).
		Auths([]user.Auth{auth}).
		Workspace(ws).
		MustBuild()

	type args struct {
		me FragmentUser
	}
	tests := []struct {
		name    string
		args    args
		want    *user.User
		wantErr bool
	}{
		{
			name: "ok",
			args: args{
				me: u,
			},
			want:    us,
			wantErr: false,
		}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FragmentToUser(tt.args.me)
			if (err != nil) != tt.wantErr {
				t.Errorf("FragmentToUser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FragmentToUser() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserByIDsNodesNodeTo(t *testing.T) {
	uid := id.NewUserID()
	ws := id.NewWorkspaceID()
	u := &UserByIDsNodesUser{
		Id:        uid.String(),
		Name:      "name",
		Email:     "email@example.com",
		Workspace: ws.String(),
		Typename:  "User",
	}
	us := user.New().ID(uid).Name("name").
		Email("email@example.com").
		Workspace(ws).
		MustBuild()

	type args struct {
		r *UserByIDsNodesUser
	}
	tests := []struct {
		name    string
		args    args
		want    *user.User
		wantErr bool
	}{
		{
			name: "ok",
			args: args{
				r: u,
			},
			want:    us,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UserByIDsNodesNodeTo(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("UserByIDsNodesNodeTo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UserByIDsNodesNodeTo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserByIDsNodesUserTo(t *testing.T) {
	uid := id.NewUserID()
	ws := id.NewWorkspaceID()
	u := &UserByIDsNodesUser{
		Id:        uid.String(),
		Name:      "name",
		Email:     "email@example.com",
		Workspace: ws.String(),
		Typename:  "User",
	}
	us := user.New().ID(uid).Name("name").
		Email("email@example.com").
		Workspace(ws).
		MustBuild()
	type args struct {
		r *UserByIDsNodesUser
	}
	tests := []struct {
		name    string
		args    args
		want    *user.User
		wantErr bool
	}{
		{
			name: "ok",
			args: args{
				r: u,
			},
			want:    us,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UserByIDsNodesUserTo(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("UserByIDsNodesUserTo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UserByIDsNodesUserTo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSimpleUserByIDsNodesNodeTo(t *testing.T) {
	uid := id.NewUserID()
	u := &UserByIDsNodesUser{
		Id:       uid.String(),
		Name:     "name",
		Email:    "email",
		Typename: "User",
	}
	type args struct {
		r UserByIDsNodesNode
	}
	tests := []struct {
		name    string
		args    args
		want    *user.Simple
		wantErr bool
	}{
		{
			name: "ok",
			args: args{
				r: u,
			},
			want: &user.Simple{
				ID:    uid,
				Name:  "name",
				Email: "email",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SimpleUserByIDsNodesNodeTo(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("SimpleUserByIDsNodesNodeTo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SimpleUserByIDsNodesNodeTo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSimpleUserByIDsNodesUserTo(t *testing.T) {
	uid := id.NewUserID()
	u := &UserByIDsNodesUser{
		Id:       uid.String(),
		Name:     "name",
		Email:    "email",
		Typename: "User",
	}
	type args struct {
		r *UserByIDsNodesUser
	}
	tests := []struct {
		name    string
		args    args
		want    *user.Simple
		wantErr bool
	}{
		{
			name: "ok",
			args: args{
				r: u,
			},
			want: &user.Simple{
				ID:    uid,
				Name:  "name",
				Email: "email",
			},
			wantErr: false,
		}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SimpleUserByIDsNodesUserTo(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("SimpleUserByIDsNodesUserTo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SimpleUserByIDsNodesUserTo() = %v, want %v", got, tt.want)
			}
		})
	}
}
