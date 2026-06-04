package adapter

import (
	"context"

	"github.com/reearth/reearth-accounts/server/internal/reearth-accounts/usecase/interfaces"
	sharedauth "github.com/reearth/reearth-accounts/server/internal/shared/auth"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/appx"
)

type ContextKey string

// AuthInfoKey is an alias of the shared auth-info context key. It is kept here
// so existing importers of adapter.AuthInfoKey keep compiling. The single
// source of truth is internal/shared/auth so lower layers can read it without
// depending on this presentation package.
var AuthInfoKey = sharedauth.AuthInfoKey

const (
	contextUser     ContextKey = "user"
	contextOperator ContextKey = "operator"
	contextUsecases ContextKey = "usecases"
	contextConfig   ContextKey = "config"
)

func AttachUser(ctx context.Context, u *user.User) context.Context {
	return context.WithValue(ctx, contextUser, u)
}

func AttachOperator(ctx context.Context, o *workspace.Operator) context.Context {
	return context.WithValue(ctx, contextOperator, o)
}

func AttachUsecases(ctx context.Context, u *interfaces.Container) context.Context {
	ctx = context.WithValue(ctx, contextUsecases, u)
	return ctx
}

func User(ctx context.Context) *user.User {
	if v := ctx.Value(contextUser); v != nil {
		if u, ok := v.(*user.User); ok {
			return u
		}
	}
	return nil
}

func Operator(ctx context.Context) *workspace.Operator {
	if v := ctx.Value(contextOperator); v != nil {
		if v2, ok := v.(*workspace.Operator); ok {
			return v2
		}
	}
	return nil
}

func GetAuthInfo(ctx context.Context) *appx.AuthInfo {
	return sharedauth.GetAuthInfo(ctx)
}

func Usecases(ctx context.Context) *interfaces.Container {
	return ctx.Value(contextUsecases).(*interfaces.Container)
}

func AttachConfig(ctx context.Context, config interface{}) context.Context {
	return context.WithValue(ctx, contextConfig, config)
}

func GetConfig(ctx context.Context) interface{} {
	return ctx.Value(contextConfig)
}
