package workspace

import (
	"github.com/reearth/reearthx/account/accountdomain/workspace"
	"github.com/reearth/reearthx/util"
)

type Workspace struct {
	id           ID
	name         string
	alias        string
	email        string
	billingEmail string
	metadata     *Metadata
	members      *workspace.Members
	location     string
}

func (w *Workspace) ID() ID {
	return w.id
}

func (w *Workspace) Name() string {
	return w.name
}

func (w *Workspace) Alias() string {
	return w.alias
}

func (w *Workspace) Email() string {
	return w.email
}

func (w *Workspace) BillingEmail() string {
	return w.billingEmail
}

func (w *Workspace) Metadata() *Metadata {
	return w.metadata
}

func (w *Workspace) Members() *workspace.Members {
	return w.members
}

func (w *Workspace) IsPersonal() bool {
	return w.members.Fixed()
}

func (w *Workspace) Location() string {
	return w.location
}

func (w *Workspace) LocationOr(def string) string {
	if w.location == "" {
		return def
	}
	return w.location
}

func (w *Workspace) Rename(name string) {
	w.name = name
}

func (w *Workspace) UpdateAlias(alias string) {
	w.alias = alias
}

func (w *Workspace) UpdateEmail(email string) {
	w.email = email
}

func (w *Workspace) UpdateBillingEmail(billingEmail string) {
	w.billingEmail = billingEmail
}

func (w *Workspace) SetMetadata(metadata *Metadata) {
	w.metadata = util.CloneRef(metadata)
}
