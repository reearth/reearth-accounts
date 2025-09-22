package app

import (
	"context"
	"net/http"

	"github.com/reearth/reearth-accounts/internal/adapter"
	"github.com/reearth/reearth-accounts/internal/usecase"
	"github.com/reearth/reearth-accounts/pkg/id"
	"github.com/reearth/reearth-accounts/pkg/user"
	"github.com/reearth/reearth-accounts/pkg/workspace"
	"github.com/reearth/reearthx/appx"
)

const (
	debugUserHeader      = "X-Reearth-Debug-User"
	debugAuthSubHeader   = "X-Reearth-Debug-Auth-Sub"
	debugAuthIssHeader   = "X-Reearth-Debug-Auth-Iss"
	debugAuthTokenHeader = "X-Reearth-Debug-Auth-Token"
	debugAuthNameHeader  = "X-Reearth-Debug-Auth-Name"
	debugAuthEmailHeader = "X-Reearth-Debug-Auth-Email"
)

func authMiddleware(cfg *ServerConfig) func(http.Handler) http.Handler {
	return appx.ContextMiddlewareBy(func(w http.ResponseWriter, req *http.Request) context.Context {
		ctx := req.Context()

		var usr *user.User

		var ai appx.AuthInfo

		// get sub from context
		if a, ok := ctx.Value(adapter.AuthInfoKey).(appx.AuthInfo); ok {
			ai = a
		}

		// debug mode: fetch user by user id add AuthInfo to context
		if cfg.Debug {
			if newCtx, dai := injectDebugAuthInfo(ctx, req); dai != nil {
				ctx = newCtx
				ai = *dai
			}
			usr = isDebugUserExists(req, cfg, ctx)
		}

		// load user by sub
		if usr == nil && ai.Sub != "" {
			existingUsr, err := cfg.Repos.User.FindBySub(ctx, ai.Sub)
			if err == nil && existingUsr != nil {
				usr = existingUsr
			}
		}

		if usr != nil {
			ctx = adapter.AttachUser(ctx, usr)
			op, err := generateUserOperator(ctx, cfg, usr)
			if err == nil {
				ctx = adapter.AttachOperator(ctx, op)
			}
		}

		return ctx
	})
}

func isDebugUserExists(req *http.Request, cfg *ServerConfig, ctx context.Context) *user.User {
	if userID := req.Header.Get(debugUserHeader); userID != "" {
		var existingUsr *user.User

		if uID, err := id.UserIDFrom(userID); err == nil {
			u, err := cfg.Repos.User.FindByID(ctx, uID)
			if err == nil {
				existingUsr = u
			}
		}

		return existingUsr

	}
	return nil
}

func generateUserOperator(ctx context.Context, cfg *ServerConfig, u *user.User) (*usecase.Operator, error) {
	if u == nil {
		return nil, nil
	}

	uid := u.ID()

	w, err := cfg.Repos.Workspace.FindByUser(ctx, uid)
	if err != nil {
		return nil, err
	}

	rw := w.FilterByUserRole(uid, workspace.RoleReader).IDs()
	ww := w.FilterByUserRole(uid, workspace.RoleWriter).IDs()
	mw := w.FilterByUserRole(uid, workspace.RoleMaintainer).IDs()
	ow := w.FilterByUserRole(uid, workspace.RoleOwner).IDs()

	return &usecase.Operator{
		User: &uid,

		ReadableWorkspaces:     rw,
		WritableWorkspaces:     ww,
		MaintainableWorkspaces: mw,
		OwningWorkspaces:       ow,
	}, nil
}

func injectDebugAuthInfo(ctx context.Context, req *http.Request) (context.Context, *appx.AuthInfo) {
	sub := req.Header.Get(debugAuthSubHeader)
	if sub == "" {
		return ctx, nil
	}
	ai := &appx.AuthInfo{
		Token: req.Header.Get(debugAuthTokenHeader),
		Sub:   sub,
		Iss:   req.Header.Get(debugAuthIssHeader),
		Name:  req.Header.Get(debugAuthNameHeader),
		Email: req.Header.Get(debugAuthEmailHeader),
	}
	return context.WithValue(ctx, adapter.AuthInfoKey, *ai), ai
}
