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

// LogVerdict logs the call once, as accepted or as refused. Only an anonymous
// call can be refused after it is resolved: a route or operation that is not
// public refuses it (ADR-0002 decision 7.1).
func (cc *ClientCall) LogVerdict(ctx context.Context, accepted bool) {
	if accepted {
		applog.Info(ctx, "auth: client call", cc.LogFields()...)
		return
	}
	applog.Warn(ctx, "auth: rejected client call", append(cc.LogFields(), applog.Reason("anonymous_call_not_public"))...)
}

func ClientCallOf(ctx context.Context) *ClientCall {
	cc, _ := ctx.Value(clientCallKey{}).(*ClientCall)
	return cc
}
