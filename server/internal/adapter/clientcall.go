package adapter

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/applog"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
)

type clientCallKey struct{}

// ClientCall is the auth layer's verdict for a request made with a registered
// client token (ADR-0002).
type ClientCall struct {
	// Issuer is the token's issuer. Client IDs are unique only within one.
	Issuer   string
	Client   *workspace.Client
	Subject  workspace.Subject
	Operator *workspace.Operator
	User     *user.User
}

// Attach is the only way a client call enters a context, so the call, its
// operator, its user and its log fields always arrive together.
func (cc *ClientCall) Attach(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, clientCallKey{}, cc)
	if cc.Operator != nil {
		ctx = AttachOperator(ctx, cc.Operator)
	}
	if cc.User != nil {
		ctx = AttachUser(ctx, cc.User)
	}
	return applog.With(ctx, cc.LogFields()...)
}

func (cc *ClientCall) LogFields() []applog.Field {
	return []applog.Field{
		applog.CallerClientID(cc.Client.ID),
		applog.CallerName(cc.Client.Name),
		applog.ActingFor(cc.Subject.String()),
	}
}

// LogClientVerdict logs a client gate's decision once per call (ADR-0002 decision 8).
func LogClientVerdict(ctx context.Context, admitted bool) {
	if ClientCallOf(ctx) == nil {
		return
	}
	if admitted {
		applog.Info(ctx, "auth: client call")
		return
	}
	applog.Warn(ctx, "auth: rejected client call", applog.Reason("not_open_to_client"))
}

func ClientCallOf(ctx context.Context) *ClientCall {
	cc, _ := ctx.Value(clientCallKey{}).(*ClientCall)
	return cc
}
