package mongodoc

import (
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/rerror"
	"go.mongodb.org/mongo-driver/bson"
)

type Type string

type Document struct {
	Type   Type
	Object bson.Raw
}

var (
	ErrInvalidObject = rerror.NewE(i18n.T("invalid object"))
	ErrInvalidDoc    = rerror.NewE(i18n.T("invalid document"))
)

func NewDocument(obj any) (doc Document, id string, err error) {
	var res any
	var ty Type

	switch m := obj.(type) {
	case *workspace.Workspace:
		ty = "workspace"
		res, id = NewWorkspace(m)
	case *user.User:
		ty = "user"
		res, id = NewUser(m)
	default:
		err = ErrInvalidObject
		return
	}

	raw, err := bson.Marshal(res)
	if err != nil {
		return
	}
	return Document{Object: raw, Type: ty}, id, nil
}

func ModelFrom(obj Document) (res any, err error) {
	switch obj.Type {
	case "workspace":
		var d *WorkspaceDocument
		if err = bson.Unmarshal(obj.Object, &d); err == nil {
			res, err = d.Model()
		}
	case "user":
		var d *UserDocument
		if err = bson.Unmarshal(obj.Object, &d); err == nil {
			res, err = d.Model()
		}
	default:
		err = ErrInvalidDoc
	}
	return
}
