//go:generate go run go.uber.org/mock/mockgen -source=repo.go -destination=mockrepo/mockrepo.go -package=mockrepo -mock_names=Repo=MockRepo

package user

import (
	"context"

	"github.com/hasura/go-graphql-client"
	"github.com/labstack/gommon/log"
	"github.com/reearth/reearth-accounts/server/pkg/gqlclient/gqlerror"
	"github.com/reearth/reearth-accounts/server/pkg/gqlclient/gqlmodel"
	"github.com/reearth/reearth-accounts/server/pkg/gqlclient/gqlutil"
	"github.com/reearth/reearth-accounts/server/pkg/user"
)

type userRepo struct {
	client *graphql.Client
}

type UpdateMeInput struct {
	Name                 *string
	Email                *string
	Lang                 *string
	Theme                *string
	Password             *string
	PasswordConfirmation *string
}

type Repo interface {
	FindMe(ctx context.Context) (*user.User, error)
	FindByID(ctx context.Context, id string) (*user.User, error)
	FindByIDs(ctx context.Context, ids []string) ([]*user.User, error)
	FindByAlias(ctx context.Context, alias string) (*user.User, error)
	FindByNameOrEmail(ctx context.Context, nameOrEmail string) (*user.User, error)
	Update(ctx context.Context, name string) error
	UpdateMe(ctx context.Context, input UpdateMeInput) (*user.User, error)
	SignupOIDC(ctx context.Context, name string, email string, sub string, secret string) (*user.User, error)
	Signup(ctx context.Context, userID, name, email, password, secret, workspaceID string, mockAuth bool) (*user.User, error)
	CreateVerification(ctx context.Context, email string) (bool, error)
	RemoveMyAuth(ctx context.Context, auth string) (*user.User, error)
	DeleteMe(ctx context.Context, userID string) error
	VerifyUser(ctx context.Context, code string) (*user.User, error)
	StartPasswordReset(ctx context.Context, email string) error
	PasswordReset(ctx context.Context, password string, token string) error
}

func NewRepo(gql *graphql.Client) Repo {
	return &userRepo{client: gql}
}

func (r *userRepo) FindMe(ctx context.Context) (*user.User, error) {
	var q findMeQuery

	if err := r.client.Query(ctx, &q, nil); err != nil {
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	id, err := user.IDFrom(string(q.Me.ID))
	if err != nil {
		log.Errorf("[FindMe] failed to convert user id: %s", q.Me.ID)
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	wid, err := user.WorkspaceIDFrom(string(q.Me.MyWorkspaceID))
	if err != nil {
		log.Errorf("[FindMe] failed to convert workspace id: %s", q.Me.MyWorkspaceID)
		return nil, gqlerror.ReturnAccountsError(ctx, err)
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
		log.Errorf("[FindByID] failed to convert user id: %s", q.User.ID)
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	wid, err := user.WorkspaceIDFrom(string(q.User.Workspace))
	if err != nil {
		log.Errorf("[FindByID] failed to convert workspace id: %s", q.User.Workspace)
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	return user.New().
		ID(uid).
		Name(string(q.User.Name)).
		Email(string(q.User.Email)).
		Workspace(wid).
		Metadata(gqlmodel.ToUserMetadata(q.User.Metadata)).
		Build()
}

func (r *userRepo) FindByIDs(ctx context.Context, ids []string) ([]*user.User, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	graphqlIDs := make([]graphql.ID, 0, len(ids))
	for _, id := range ids {
		graphqlIDs = append(graphqlIDs, graphql.ID(id))
	}

	var q findUsersByIDsQuery
	vars := map[string]interface{}{
		"ids": graphqlIDs,
	}
	if err := r.client.Query(ctx, &q, vars); err != nil {
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	users := make([]*user.User, 0, len(q.Users))
	for _, u := range q.Users {
		uid, err := user.IDFrom(string(u.ID))
		if err != nil {
			log.Errorf("[FindByIDs] failed to convert user id: %s", u.ID)
			return nil, gqlerror.ReturnAccountsError(ctx, err)
		}

		wid, err := user.WorkspaceIDFrom(string(u.Workspace))
		if err != nil {
			log.Errorf("[FindByIDs] failed to convert workspace id: %s", u.Workspace)
			return nil, gqlerror.ReturnAccountsError(ctx, err)
		}

		auths := gqlutil.ToStringSlice(u.Auths)
		auths2 := make([]user.Auth, len(auths))
		for i, auth := range auths {
			auths2[i] = user.AuthFrom(auth)
		}

		userObj, err := user.New().
			ID(uid).
			Name(string(u.Name)).
			Email(string(u.Email)).
			Workspace(wid).
			Auths(auths2).
			Metadata(gqlmodel.ToUserMetadata(u.Metadata)).
			Build()

		if err != nil {
			return nil, gqlerror.ReturnAccountsError(ctx, err)
		}

		users = append(users, userObj)
	}

	return users, nil
}

func (r *userRepo) FindByAlias(ctx context.Context, alias string) (*user.User, error) {
	var q findByAliasQuery
	vars := map[string]interface{}{
		"alias": graphql.String(alias),
	}
	if err := r.client.Query(ctx, &q, vars); err != nil {
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	uid, err := user.IDFrom(string(q.User.ID))
	if err != nil {
		log.Errorf("[FindByAlias] failed to convert user id: %s", q.User.ID)
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	return user.New().
		ID(uid).
		Name(string(q.User.Name)).
		Alias(string(q.User.Alias)).
		Email(string(q.User.Email)).
		Build()
}

func (r *userRepo) FindByNameOrEmail(ctx context.Context, nameOrEmail string) (*user.User, error) {
	var q userByNameOrEmailQuery
	vars := map[string]interface{}{
		"nameOrEmail": graphql.String(nameOrEmail),
	}
	if err := r.client.Query(ctx, &q, vars); err != nil {
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	uid, err := user.IDFrom(string(q.User.ID))
	if err != nil {
		log.Errorf("[FindByNameOrEmail] failed to convert user id: %s", q.User.ID)
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	return user.New().
		ID(uid).
		Name(string(q.User.Name)).
		Email(string(q.User.Email)).
		Build()
}

// Deprecated: Use FindByNameOrEmail instead
func (r *userRepo) FindByNameEmail(ctx context.Context, nameOrEmail string) (*user.User, error) {
	return r.FindByNameOrEmail(ctx, nameOrEmail)
}

func (r *userRepo) FindUsersByIDsWithPagination(ctx context.Context, id []string, alias string, page, size int64) (user.List, int, error) {
	var q FindUsersByIDsWithPaginationQuery
	vars := map[string]interface{}{
		"ids":   gqlutil.ToIDSlice(id),
		"alias": graphql.String(alias),
		"page":  graphql.Int(page),
		"size":  graphql.Int(size),
	}
	if err := r.client.Query(ctx, &q, vars); err != nil {
		return nil, 0, gqlerror.ReturnAccountsError(ctx, err)
	}

	users := gqlmodel.ToUsers(ctx, q.FindUsersByIDsWithPagination.Users)
	return users, q.FindUsersByIDsWithPagination.TotalCount, nil
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

func (r *userRepo) UpdateMe(ctx context.Context, input UpdateMeInput) (*user.User, error) {
	var m updateMeFullMutation

	vars := map[string]interface{}{
		"name":                 (*graphql.String)(nil),
		"email":                (*graphql.String)(nil),
		"lang":                 (*gqlmodel.Lang)(nil),
		"theme":                (*gqlmodel.Theme)(nil),
		"password":             (*graphql.String)(nil),
		"passwordConfirmation": (*graphql.String)(nil),
	}

	if input.Name != nil {
		name := graphql.String(*input.Name)
		vars["name"] = &name
	}
	if input.Email != nil {
		email := graphql.String(*input.Email)
		vars["email"] = &email
	}
	if input.Lang != nil {
		lang := gqlmodel.Lang(*input.Lang)
		vars["lang"] = &lang
	}
	if input.Theme != nil {
		theme := gqlmodel.Theme(*input.Theme)
		vars["theme"] = &theme
	}
	if input.Password != nil {
		password := graphql.String(*input.Password)
		vars["password"] = &password
	}
	if input.PasswordConfirmation != nil {
		passwordConfirmation := graphql.String(*input.PasswordConfirmation)
		vars["passwordConfirmation"] = &passwordConfirmation
	}

	if err := r.client.Mutate(ctx, &m, vars); err != nil {
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	id, err := user.IDFrom(string(m.UpdateMe.Me.ID))
	if err != nil {
		log.Errorf("[UpdateMe] failed to convert user id: %s", m.UpdateMe.Me.ID)
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	wid, err := user.WorkspaceIDFrom(string(m.UpdateMe.Me.MyWorkspaceID))
	if err != nil {
		log.Errorf("[UpdateMe] failed to convert workspace id: %s", m.UpdateMe.Me.MyWorkspaceID)
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	auths := gqlutil.ToStringSlice(m.UpdateMe.Me.Auths)
	auths2 := make([]user.Auth, len(auths))
	for i, auth := range auths {
		auths2[i] = user.AuthFrom(auth)
	}

	return user.New().
		ID(id).
		Name(string(m.UpdateMe.Me.Name)).
		Alias(string(m.UpdateMe.Me.Alias)).
		Email(string(m.UpdateMe.Me.Email)).
		Metadata(gqlmodel.ToUserMetadata(m.UpdateMe.Me.Metadata)).
		Workspace(wid).
		Auths(auths2).
		Build()
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
		log.Errorf("[SignupOIDC] failed to convert user id: %s", m.SignupOIDC.User.ID)
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	return user.New().
		ID(uid).
		Name(string(m.SignupOIDC.User.Name)).
		Email(string(m.SignupOIDC.User.Email)).
		Build()
}

func (r *userRepo) Signup(ctx context.Context, userID, name, email, password, secret, workspaceID string, mockAuth bool) (*user.User, error) {
	if userID == "" {
		return r.SignupNoID(ctx, name, email, password, secret, workspaceID, mockAuth)
	}

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
	vars["mockAuth"] = graphql.Boolean(mockAuth)

	if err := r.client.Mutate(ctx, &m, vars); err != nil {
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	uid, err := user.IDFrom(string(m.Signup.User.ID))
	if err != nil {
		log.Errorf("[Signup] failed to convert user id: %s", m.Signup.User.ID)
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	return user.New().
		ID(uid).
		Name(string(m.Signup.User.Name)).
		Email(string(m.Signup.User.Email)).
		Build()
}

func (r *userRepo) SignupNoID(ctx context.Context, name, email, password, secret, workspaceID string, mockAuth bool) (*user.User, error) {
	var m signupMutationNoID
	vars := map[string]interface{}{}

	vars["name"] = graphql.String(name)
	vars["email"] = graphql.String(email)
	vars["password"] = graphql.String(password)
	vars["secret"] = graphql.String(secret)
	vars["mockAuth"] = graphql.Boolean(mockAuth)

	if err := r.client.Mutate(ctx, &m, vars); err != nil {
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	uid, err := user.IDFrom(string(m.Signup.User.ID))
	if err != nil {
		log.Errorf("[SignupNoID] failed to convert user id: %s", m.Signup.User.ID)
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	return user.New().
		ID(uid).
		Name(string(m.Signup.User.Name)).
		Email(string(m.Signup.User.Email)).
		Build()
}

func (r *userRepo) CreateVerification(ctx context.Context, email string) (bool, error) {
	var m createVerificationMutation
	vars := map[string]interface{}{
		"email": graphql.String(email),
	}
	if err := r.client.Mutate(ctx, &m, vars); err != nil {
		return false, gqlerror.ReturnAccountsError(ctx, err)
	}

	return *m.CreateVerification, nil
}

func (r *userRepo) DeleteMe(ctx context.Context, userID string) error {
	var m deleteMeMutation
	vars := map[string]interface{}{
		"userId": graphql.ID(userID),
	}
	return r.client.Mutate(ctx, &m, vars)
}

func (r *userRepo) RemoveMyAuth(ctx context.Context, auth string) (*user.User, error) {
	var m removeMyAuthMutation
	vars := map[string]interface{}{
		"auth": graphql.String(auth),
	}
	if err := r.client.Mutate(ctx, &m, vars); err != nil {
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	id, err := user.IDFrom(string(m.RemoveMyAuth.Me.ID))
	if err != nil {
		log.Errorf("[RemoveMyAuth] failed to convert user id: %s", m.RemoveMyAuth.Me.ID)
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	wid, err := user.WorkspaceIDFrom(string(m.RemoveMyAuth.Me.MyWorkspaceID))
	if err != nil {
		log.Errorf("[RemoveMyAuth] failed to convert workspace id: %s", m.RemoveMyAuth.Me.MyWorkspaceID)
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	auths := gqlutil.ToStringSlice(m.RemoveMyAuth.Me.Auths)
	auths2 := make([]user.Auth, len(auths))
	for i, auth := range auths {
		auths2[i] = user.AuthFrom(auth)
	}

	return user.New().
		ID(id).
		Name(string(m.RemoveMyAuth.Me.Name)).
		Alias(string(m.RemoveMyAuth.Me.Alias)).
		Email(string(m.RemoveMyAuth.Me.Email)).
		Metadata(gqlmodel.ToUserMetadata(m.RemoveMyAuth.Me.Metadata)).
		Workspace(wid).
		Auths(auths2).
		Build()
}

func (r *userRepo) VerifyUser(ctx context.Context, code string) (*user.User, error) {
	if code == "" {
		return nil, nil
	}

	in := VerifyUserInput{
		Code: graphql.String(code),
	}

	var m verifyUserMutation
	vars := map[string]interface{}{
		"input": in,
	}

	if err := r.client.Mutate(ctx, &m, vars); err != nil {
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	uid, err := user.IDFrom(string(m.VerifyUser.User.ID))
	if err != nil {
		log.Errorf("[VerifyUser] failed to convert user id: %s", m.VerifyUser.User.ID)
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	wid, err := user.WorkspaceIDFrom(string(m.VerifyUser.User.Workspace))
	if err != nil {
		log.Errorf("[VerifyUser] failed to convert workspace id: %s", m.VerifyUser.User.Workspace)
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	auths := gqlutil.ToStringSlice(m.VerifyUser.User.Auths)
	auths2 := make([]user.Auth, len(auths))
	for i, auth := range auths {
		auths2[i] = user.AuthFrom(auth)
	}

	return user.New().
		ID(uid).
		Name(string(m.VerifyUser.User.Name)).
		Email(string(m.VerifyUser.User.Email)).
		Workspace(wid).
		Auths(auths2).
		Metadata(gqlmodel.ToUserMetadata(m.VerifyUser.User.Metadata)).
		Build()
}

func (r *userRepo) StartPasswordReset(ctx context.Context, email string) error {
	if email == "" {
		return nil
	}

	in := StartPasswordResetInput{
		Email: graphql.String(email),
	}

	var m startPasswordResetMutation
	vars := map[string]interface{}{
		"input": in,
	}

	if err := r.client.Mutate(ctx, &m, vars); err != nil {
		return gqlerror.ReturnAccountsError(ctx, err)
	}

	return nil
}

func (r *userRepo) PasswordReset(ctx context.Context, password string, token string) error {
	if password == "" || token == "" {
		return nil
	}

	in := PasswordResetInput{
		Password: graphql.String(password),
		Token:    graphql.String(token),
	}

	var m passwordResetMutation
	vars := map[string]interface{}{
		"input": in,
	}

	if err := r.client.Mutate(ctx, &m, vars); err != nil {
		return gqlerror.ReturnAccountsError(ctx, err)
	}

	return nil
}
