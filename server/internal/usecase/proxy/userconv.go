package proxy

import (
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/util"
)

func UserByIDsResponseTo(r *UserByIDsResponse, err error) ([]*user.User, error) {
	if err != nil || r == nil {
		return nil, err
	}
	return util.TryMap(r.Nodes, UserByIDsNodesNodeTo)
}

func SimpleUserByIDsResponseTo(r *UserByIDsResponse, err error) ([]*user.Simple, error) {
	if err != nil || r == nil {
		return nil, err
	}
	return util.TryMap(r.Nodes, SimpleUserByIDsNodesNodeTo)
}

func MeToUser(me FragmentMe) (*user.User, error) {
	id, err := user.IDFrom(me.Id)
	if err != nil {
		return nil, err
	}
	wid, err := user.WorkspaceIDFrom(me.MyWorkspaceId)
	if err != nil {
		return nil, err
	}

	auths := make([]user.Auth, len(me.Auths))
	for i := range me.Auths {
		auths[i] = user.AuthFrom(me.Auths[i])
	}

	metadata := user.NewMetadata()
	metadata.LangFrom(me.Metadata.Lang)
	metadata.SetDescription(me.Metadata.Description)
	metadata.SetPhotoURL(me.Metadata.PhotoURL)
	metadata.SetTheme(user.ThemeFrom(me.Metadata.Theme))
	metadata.SetWebsite(me.Metadata.Website)

	u, err := user.New().ID(id).Name(me.Name).
		Email(me.Email).
		Metadata(metadata).
		Auths(auths).
		Workspace(wid).Build()
	if err != nil {
		return nil, err
	}

	return u, nil
}

func FragmentToUser(me FragmentUser) (*user.User, error) {
	id, err := user.IDFrom(me.Id)
	if err != nil {
		return nil, err
	}
	wid, err := user.WorkspaceIDFrom(me.Workspace)
	if err != nil {
		return nil, err
	}

	auths := make([]user.Auth, len(me.Auths))
	for i := range me.Auths {
		auths[i] = user.AuthFrom(me.Auths[i])
	}

	metadata := user.NewMetadata()
	metadata.LangFrom(me.Metadata.Lang)
	metadata.SetDescription(me.Metadata.Description)
	metadata.SetPhotoURL(me.Metadata.PhotoURL)
	metadata.SetTheme(user.ThemeFrom(me.Metadata.Theme))
	metadata.SetWebsite(me.Metadata.Website)

	u, err := user.New().ID(id).Name(me.Name).
		Alias(me.Alias).
		Email(me.Email).
		Metadata(metadata).
		Auths(auths).
		Workspace(wid).Build()
	if err != nil {
		return nil, err
	}

	return u, nil
}

func UserByIDsNodesNodeTo(r UserByIDsNodesNode) (*user.User, error) {
	if r == nil {
		return nil, nil
	}
	u, ok := r.(*UserByIDsNodesUser)
	if !ok || u == nil {
		return nil, nil
	}
	return UserByIDsNodesUserTo(u)
}

func UserByIDsNodesUserTo(r *UserByIDsNodesUser) (*user.User, error) {
	if r == nil {
		return nil, nil
	}
	id, err := user.IDFrom(r.Id)
	if err != nil {
		return nil, err
	}
	wid, err := user.WorkspaceIDFrom(r.Workspace)
	if err != nil {
		return nil, err
	}

	auths := make([]user.Auth, len(r.Auths))
	for i := range r.Auths {
		auths[i] = user.AuthFrom(r.Auths[i])
	}

	metadata := user.NewMetadata()
	metadata.LangFrom(r.Metadata.Lang)
	metadata.SetDescription(r.Metadata.Description)
	metadata.SetPhotoURL(r.Metadata.PhotoURL)
	metadata.SetTheme(user.ThemeFrom(r.Metadata.Theme))
	metadata.SetWebsite(r.Metadata.Website)

	return user.New().ID(id).Name(r.Name).
		Email(r.Email).
		Metadata(metadata).
		Auths(auths).
		Workspace(wid).Build()
}

func SimpleUserByIDsNodesNodeTo(r UserByIDsNodesNode) (*user.Simple, error) {
	if r == nil {
		return nil, nil
	}
	u, ok := r.(*UserByIDsNodesUser)
	if !ok || u == nil {
		return nil, nil
	}
	return SimpleUserByIDsNodesUserTo(u)
}

func SimpleUserByIDsNodesUserTo(r *UserByIDsNodesUser) (*user.Simple, error) {
	if r == nil {
		return nil, nil
	}
	id, err := user.IDFrom(r.Id)
	if err != nil {
		return nil, err
	}
	return &user.Simple{
		ID:    id,
		Name:  r.Name,
		Email: r.Email,
	}, nil
}
