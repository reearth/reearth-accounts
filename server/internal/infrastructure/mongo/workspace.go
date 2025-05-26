package mongo

import (
	"context"

	"github.com/reearth/reearth-accounts/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearth-accounts/pkg/workspace"
	"github.com/reearth/reearthx/account/accountdomain"
	"github.com/reearth/reearthx/account/accountusecase/accountrepo"
	"github.com/reearth/reearthx/mongox"
	"github.com/reearth/reearthx/rerror"
	"go.mongodb.org/mongo-driver/bson"
)

type Workspace struct {
	client *mongox.ClientCollection
	f      accountrepo.WorkspaceFilter
}

func NewWorkspace(client *mongox.Client) *Workspace {
	return &Workspace{
		client: client.Collection("workspace"),
	}
}

func (w *Workspace) FindByID(ctx context.Context, id workspace.ID) (*workspace.Workspace, error) {
	if !w.f.CanRead(accountdomain.WorkspaceID(id)) {
		return nil, rerror.ErrNotFound
	}

	return w.findOne(ctx, bson.M{"id": id.String()})
}

func (w *Workspace) Save(ctx context.Context, workspace *workspace.Workspace) error {
	if !w.f.CanWrite(accountdomain.WorkspaceID(workspace.ID())) {
		return accountrepo.ErrOperationDenied
	}

	doc, id := mongodoc.NewWorkspace(workspace)
	return w.client.SaveOne(ctx, id, doc)
}

func (w *Workspace) findOne(ctx context.Context, filter any) (*workspace.Workspace, error) {
	c := mongodoc.NewWorkspaceConsumer()
	filter = w.f.Filter(filter)
	if err := w.client.FindOne(ctx, filter, c); err != nil {
		return nil, err
	}
	return c.Result[0], nil
}
