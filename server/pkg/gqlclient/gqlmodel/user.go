package gqlmodel

import (
	"context"

	"github.com/hasura/go-graphql-client"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/log"
	"golang.org/x/text/language"
)

type User struct {
	ID        graphql.ID       `json:"id" graphql:"id"`
	Alias     graphql.String   `json:"alias" graphql:"alias"`
	Name      graphql.String   `json:"name" graphql:"name"`
	Email     graphql.String   `json:"email" graphql:"email"`
	Host      *graphql.String  `json:"host,omitempty" graphql:"host"`
	Workspace graphql.ID       `json:"workspace" graphql:"workspace"`
	Auths     []graphql.String `json:"auths" graphql:"auths"`
	Metadata  UserMetadata     `json:"metadata" graphql:"metadata"`
}

func ToUser(ctx context.Context, u User) (*user.User, error) {
	id, err := user.IDFrom(string(u.ID))
	if err != nil {
		log.Errorf("[ToUser] failed to convert user id: %s", u.ID)
		return nil, err
	}

	workspaceID, err := user.WorkspaceIDFrom(string(u.Workspace))
	if err != nil {
		log.Errorf("[ToUser] failed to convert workspace id: %s", u.Workspace)
		return nil, err
	}

	return user.New().
		ID(id).
		Alias(string(u.Alias)).
		Name(string(u.Name)).
		Email(string(u.Email)).
		Metadata(user.MetadataFrom(
			string(u.Metadata.PhotoURL),
			string(u.Metadata.Description),
			string(u.Metadata.Website),
			language.Make(string(u.Metadata.Lang)),
			user.ThemeFrom(string(u.Metadata.Theme)),
		)).
		Workspace(workspaceID).
		MustBuild(), nil
}

func ToUsers(ctx context.Context, gqlUsers []User) user.List {
	users := make(user.List, 0, len(gqlUsers))
	for _, u := range gqlUsers {
		if usr, err := ToUser(ctx, u); err != nil {
			log.Errorf("[ToUsers] failed to convert user: %s", err)
			continue
		} else {
			users = append(users, usr)
		}
	}
	return users
}
