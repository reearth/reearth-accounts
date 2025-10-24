package user

import (
	"context"

	"github.com/hasura/go-graphql-client"
	"github.com/reearth/reearth-accounts/server/pkg/gqlclient/gqlerror"
	"github.com/reearth/reearth-accounts/server/pkg/gqlclient/gqlmodel"
	"github.com/reearth/reearth-accounts/server/pkg/gqlclient/gqlutil"
	"github.com/reearth/reearth-accounts/server/pkg/user"
)

type userRepo struct {
	client *graphql.Client
}

type UserRepo interface {
	FindMe(ctx context.Context) (*user.User, error)
	FindByID(ctx context.Context, id string) (*user.User, error)
	FindByAlias(ctx context.Context, name string) (*user.User, error)
	Update(ctx context.Context, name string) error
	SignupOIDC(ctx context.Context, name string, email string, sub string, secret string) (*user.User, error)
	Signup(ctx context.Context, userID, name, email, password, secret, workspaceID string) (*user.User, error)
}

func NewRepo(gql *graphql.Client) UserRepo {
	return &userRepo{client: gql}
}

func (r *userRepo) FindMe(ctx context.Context) (*user.User, error) {
	var q findMeQuery

	if err := r.client.Query(ctx, &q, nil); err != nil {
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	id, err := user.IDFrom(string(q.Me.ID))
	if err != nil {
		return nil, err
	}

	wid, err := user.WorkspaceIDFrom(string(q.Me.MyWorkspaceID))
	if err != nil {
		return nil, err
	}

	auths := gqlutil.ToStringSlice(q.Me.Auths)
	auths2 := make([]user.Auth, len(auths))
	for i, auth := range auths {
		auths2[i] = user.AuthFrom(auth)
	}

	return user.New().
		ID(id).
		Name(string(q.Me.Name)).
		Alias(string(q.Me.Alias)).
		Email(string(q.Me.Email)).
		Metadata(gqlmodel.ToUserMetadata(q.Me.Metadata)).
		Workspace(wid).
		Auths(auths2).
		Build()
}

func (r *userRepo) FindByID(ctx context.Context, id string) (*user.User, error) {
	var q findByIDQuery
	vars := map[string]interface{}{
		"id": graphql.ID(id),
	}
	if err := r.client.Query(ctx, &q, vars); err != nil {
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	uid, err := user.IDFrom(string(q.User.ID))
	if err != nil {
		return nil, err
	}

	wid, err := user.WorkspaceIDFrom(string(q.User.Workspace))
	if err != nil {
		return nil, err
	}

	return user.New().
		ID(uid).
		Name(string(q.User.Name)).
		Email(string(q.User.Email)).
		Workspace(wid).
		Metadata(gqlmodel.ToUserMetadata(q.User.Metadata)).
		Build()
}

func (r *userRepo) FindByAlias(ctx context.Context, name string) (*user.User, error) {
	var q findByNameQuery
	vars := map[string]interface{}{
		"nameOrEmail": graphql.String(name),
	}
	if err := r.client.Query(ctx, &q, vars); err != nil {
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	uid, err := user.IDFrom(string(q.User.ID))
	if err != nil {
		return nil, err
	}

	return user.New().
		ID(uid).
		Name(string(q.User.Name)).
		Email(string(q.User.Email)).
		Build()
}

// TODO: Extend the Account server's UpdateMeInput to support alias, photoURL, website, and description.
// This function currently only updates the 'name' field due to server-side limitations.
func (r *userRepo) Update(ctx context.Context, name string) error {
	var m updateMeMutation
	vars := map[string]interface{}{
		"name": graphql.String(name),
	}
	return r.client.Mutate(ctx, &m, vars)
}

func (r *userRepo) SignupOIDC(ctx context.Context, name string, email string, sub string, secret string) (*user.User, error) {
	var m signupOIDCMutation
	vars := map[string]interface{}{
		"name":   graphql.String(name),
		"email":  graphql.String(email),
		"sub":    graphql.String(sub),
		"secret": graphql.String(secret),
	}
	if err := r.client.Mutate(ctx, &m, vars); err != nil {
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	uid, err := user.IDFrom(string(m.SignupOIDC.User.ID))
	if err != nil {
		return nil, err
	}

	return user.New().
		ID(uid).
		Name(string(m.SignupOIDC.User.Name)).
		Email(string(m.SignupOIDC.User.Email)).
		Build()
}

func (r *userRepo) Signup(ctx context.Context, userID, name, email, password, secret, workspaceID string) (*user.User, error) {
	var m signupMutation
	vars := map[string]interface{}{}

	if userID != "" {
		vars["id"] = graphql.ID(userID)
	}

	if workspaceID != "" {
		vars["workspaceID"] = graphql.ID(workspaceID)
	}

	vars["name"] = graphql.String(name)
	vars["email"] = graphql.String(email)
	vars["password"] = graphql.String(password)
	vars["secret"] = graphql.String(secret)

	if err := r.client.Mutate(ctx, &m, vars); err != nil {
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	uid, err := user.IDFrom(string(m.Signup.User.ID))
	if err != nil {
		return nil, err
	}

	return user.New().
		ID(uid).
		Name(string(m.Signup.User.Name)).
		Email(string(m.Signup.User.Email)).
		Build()
}
