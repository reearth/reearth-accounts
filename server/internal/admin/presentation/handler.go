package presentation

import (
	"github.com/labstack/echo/v4"
	adminuserhandler "github.com/reearth/reearth-accounts/server/internal/admin/presentation/handler/adminuser"
	"github.com/reearth/reearth-accounts/server/internal/admin/presentation/handler/auth"
	"github.com/reearth/reearth-accounts/server/internal/admin/presentation/handler/user"
	workspacehandler "github.com/reearth/reearth-accounts/server/internal/admin/presentation/handler/workspace"
	mw "github.com/reearth/reearth-accounts/server/internal/admin/presentation/middleware"
)

// Handler aggregates all resource-specific admin handlers plus the middlewares.
type Handler struct {
	AdminUser       *adminuserhandler.Handler
	Auth            *auth.Handler
	User            *user.Handler
	Workspace       *workspacehandler.Handler
	AuthMw          echo.MiddlewareFunc
	SessionMw       mw.SessionMiddleware
	RequireApproved mw.RequireApprovedMiddleware
}

// NewHandler is a Wire provider that assembles the top-level admin Handler.
func NewHandler(
	adminUserHandler *adminuserhandler.Handler,
	authHandler *auth.Handler,
	userHandler *user.Handler,
	workspaceHandler *workspacehandler.Handler,
	authMw echo.MiddlewareFunc,
	sessionMw mw.SessionMiddleware,
	requireApproved mw.RequireApprovedMiddleware,
) *Handler {
	return &Handler{
		AdminUser:       adminUserHandler,
		Auth:            authHandler,
		User:            userHandler,
		Workspace:       workspaceHandler,
		AuthMw:          authMw,
		SessionMw:       sessionMw,
		RequireApproved: requireApproved,
	}
}
