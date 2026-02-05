package user

import (
	"github.com/hasura/go-graphql-client"
	"github.com/reearth/reearth-accounts/server/pkg/gqlclient/gqlmodel"
)

type findMeQuery struct {
	Me gqlmodel.Me `graphql:"me"`
}

type findByIDQuery struct {
	User gqlmodel.User `graphql:"user(id: $id)"`
}

type findUsersByIDsQuery struct {
	Users []gqlmodel.User `graphql:"findUsersByIDs(ids: $ids)"`
}

type userByNameOrEmailQuery struct {
	User gqlmodel.UserSimple `graphql:"userByNameOrEmail(nameOrEmail: $nameOrEmail)"`
}

type findByAliasQuery struct {
	User struct {
		ID        graphql.ID       `json:"id" graphql:"id"`
		Name      graphql.String   `json:"name" graphql:"name"`
		Email     graphql.String   `json:"email" graphql:"email"`
		Alias     graphql.String   `json:"alias" graphql:"alias"`
		Host      *graphql.String  `json:"host,omitempty" graphql:"host"`
		Workspace graphql.ID       `json:"workspace" graphql:"workspace"`
		Auths     []graphql.String `json:"auths" graphql:"auths"`
	} `graphql:"findUserByAlias(alias: $alias)"`
}

type FindUsersByIDsWithPaginationQuery struct {
	FindUsersByIDsWithPagination struct {
		Users      []gqlmodel.User `graphql:"users"`
		TotalCount int             `graphql:"totalCount"`
	} `graphql:"findUsersByIDsWithPagination(ids: $ids, alias: $alias, pagination: {page: $page, size: $size})"`
}

type updateMeMutation struct {
	UpdateMe struct {
		Me gqlmodel.Me
	} `graphql:"updateMe(input: {name: $name})"`
}

type updateMeFullMutation struct {
	UpdateMe struct {
		Me gqlmodel.Me
	} `graphql:"updateMe(input: {name: $name, email: $email, lang: $lang, theme: $theme, password: $password, passwordConfirmation: $passwordConfirmation})"`
}

type signupOIDCMutation struct {
	SignupOIDC struct {
		User gqlmodel.User
	} `graphql:"signupOIDC(input: {name: $name, email: $email, sub: $sub, secret: $secret})"`
}

type signupMutation struct {
	Signup struct {
		User gqlmodel.User
	} `graphql:"signup(input: {name: $name, email: $email, password: $password, secret: $secret, id: $id, workspaceID: $workspaceID, mockAuth: $mockAuth})"`
}

type signupMutationNoID struct {
	Signup struct {
		User gqlmodel.User
	} `graphql:"signup(input: {name: $name, email: $email, password: $password, secret: $secret, mockAuth: $mockAuth})"`
}

type createVerificationMutation struct {
	CreateVerification *bool `graphql:"createVerification(input: {email: $email})"`
}

type deleteMeMutation struct {
	DeleteMe struct {
		UserID graphql.ID `json:"userId"`
	} `graphql:"deleteMe(input: {userId: $userId})"`
}

type removeMyAuthMutation struct {
	RemoveMyAuth struct {
		Me gqlmodel.Me
	} `graphql:"removeMyAuth(input: {auth: $auth})"`
}

type VerifyUserInput struct {
	Code graphql.String `json:"code"`
}

type verifyUserMutation struct {
	VerifyUser struct {
		User gqlmodel.User `graphql:"user"`
	} `graphql:"verifyUser(input: $input)"`
}

type StartPasswordResetInput struct {
	Email graphql.String `json:"email"`
}

type startPasswordResetMutation struct {
	StartPasswordReset graphql.Boolean `graphql:"startPasswordReset(input: $input)"`
}

type PasswordResetInput struct {
	Password graphql.String `json:"password"`
	Token    graphql.String `json:"token"`
}

type passwordResetMutation struct {
	PasswordReset graphql.Boolean `graphql:"passwordReset(input: $input)"`
}
