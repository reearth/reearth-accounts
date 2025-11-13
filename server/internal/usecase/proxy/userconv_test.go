package proxy

import (
	"errors"
	"reflect"
	"testing"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/user"
)

func TestUserByIDsResponseTo(t *testing.T) {
	uid := id.NewUserID()
	ws := id.NewWorkspaceID()
	u := &UserByIDsNodesUser{
		Typename:  "User",
		Id:        uid.String(),
		Name:      "name",
		Email:     "email@example.com",
		Workspace: ws.String(),
		Auths:     nil,
		Metadata: UserByIDsNodesUserMetadata{
			Description: "description",
			Lang:        "ja",
			PhotoURL:    "https://example.com/photo.jpg",
			Theme:       "dark",
			Website:     "https://example.com",
		},
	}
	metadata := user.NewMetadata()
	metadata.LangFrom(u.Metadata.Lang)
	metadata.SetDescription(u.Metadata.Description)
	metadata.SetPhotoURL(u.Metadata.PhotoURL)
	metadata.SetTheme(user.ThemeFrom(u.Metadata.Theme))
	metadata.SetWebsite(u.Metadata.Website)

	us := user.New().ID(uid).Name("name").
		Email("email@example.com").
		Workspace(ws).
		Metadata(metadata).
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

	metadata := user.NewMetadata()
	metadata.LangFrom("ja")
	metadata.SetDescription("description")
	metadata.SetPhotoURL("https://example.com/photo.jpg")
	metadata.SetTheme("dark")
	metadata.SetWebsite("https://example.com")

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
					Id:    uid.String(),
					Name:  "name",
					Email: "test@exmple.com",
					Metadata: FragmentMeMetadataUserMetadata{
						Description: "description",
						Lang:        "ja",
						PhotoURL:    "https://example.com/photo.jpg",
						Theme:       "dark",
						Website:     "https://example.com",
					},
					MyWorkspaceId: wid.String(),
					Auths:         []string{"foo|bar"},
				},
			},
			want: user.New().ID(uid).Name("name").
				Email("test@exmple.com").Metadata(metadata).
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
		Auths:     []string{"sub"},
		Metadata: FragmentUserMetadata{
			Description: "description",
			Lang:        "ja",
			PhotoURL:    "https://example.com/photo.jpg",
			Theme:       "dark",
			Website:     "https://example.com",
		},
	}
	auth := user.AuthFrom("sub")

	metadata := user.NewMetadata()
	metadata.LangFrom(u.Metadata.Lang)
	metadata.SetDescription(u.Metadata.Description)
	metadata.SetPhotoURL(u.Metadata.PhotoURL)
	metadata.SetTheme(user.ThemeFrom(u.Metadata.Theme))
	metadata.SetWebsite(u.Metadata.Website)

	us := user.New().ID(uid).Name("name").
		Email("email@example.com").
		Metadata(metadata).
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
		Metadata: UserByIDsNodesUserMetadata{
			Description: "description",
			Lang:        "ja",
			PhotoURL:    "https://example.com/photo.jpg",
			Theme:       "dark",
			Website:     "https://example.com",
		},
	}
	metadata := user.NewMetadata()
	metadata.LangFrom(u.Metadata.Lang)
	metadata.SetDescription(u.Metadata.Description)
	metadata.SetPhotoURL(u.Metadata.PhotoURL)
	metadata.SetTheme(user.ThemeFrom(u.Metadata.Theme))
	metadata.SetWebsite(u.Metadata.Website)

	us := user.New().ID(uid).Name("name").
		Email("email@example.com").
		Workspace(ws).
		Metadata(metadata).
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
		Metadata: UserByIDsNodesUserMetadata{
			Description: "description",
			Lang:        "ja",
			PhotoURL:    "https://example.com/photo.jpg",
			Theme:       "dark",
			Website:     "https://example.com",
		},
	}
	metadata := user.NewMetadata()
	metadata.LangFrom(u.Metadata.Lang)
	metadata.SetDescription(u.Metadata.Description)
	metadata.SetPhotoURL(u.Metadata.PhotoURL)
	metadata.SetTheme(user.ThemeFrom(u.Metadata.Theme))
	metadata.SetWebsite(u.Metadata.Website)
	us := user.New().ID(uid).Name("name").
		Email("email@example.com").
		Workspace(ws).
		Metadata(metadata).
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
