package workspace

import (
	"fmt"
	"time"

	"github.com/reearth/reearthx/util"
)

type Workspace struct {
	id        ID
	name      string
	alias     string
	email     string
	metadata  Metadata
	members   *Members
	policy    *PolicyID
	createdAt *time.Time
	createdBy *UserID
	updatedAt time.Time
	deletedAt *time.Time
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

func (w *Workspace) Metadata() *Metadata {
	return &w.metadata
}

func (w *Workspace) Members() *Members {
	return w.members
}

func (w *Workspace) IsPersonal() bool {
	return w.members.Fixed()
}

func (w *Workspace) Rename(name string) {
	w.name = name
	w.updatedAt = time.Now()
}

func (w *Workspace) UpdateAlias(alias string) {
	w.alias = alias
	w.updatedAt = time.Now()
}

func (w *Workspace) UpdateEmail(email string) {
	w.email = email
	w.updatedAt = time.Now()
}

func (w *Workspace) SetMetadata(metadata Metadata) {
	w.metadata = metadata
	w.updatedAt = time.Now()
}

func (w *Workspace) Policy() *PolicyID {
	return util.CloneRef(w.policy)
}

func (w *Workspace) PolicytOr(def PolicyID) PolicyID {
	if w.policy == nil {
		return def
	}
	return *w.policy
}

func (w *Workspace) SetPolicy(policy *PolicyID) {
	w.policy = util.CloneRef(policy)
	w.updatedAt = time.Now()
}

func (w *Workspace) CreatedAt() *time.Time {
	return w.createdAt
}

func (w *Workspace) CreatedBy() *UserID {
	return w.createdBy
}

func (w *Workspace) Delete() {
	now := time.Now()
	w.deletedAt = &now
	w.updatedAt = time.Now()
}

func (w *Workspace) DeletedAt() *time.Time {
	return w.deletedAt
}

func (w *Workspace) DeleteIntegrations(iids IntegrationIDList) error {
	err := w.members.DeleteIntegrations(iids)
	if err != nil {
		return err
	}
	w.updatedAt = time.Now()
	return nil
}

func (w *Workspace) IsDeleted() bool {
	return w.deletedAt != nil
}

func (w *Workspace) Restore() {
	w.deletedAt = nil
	w.updatedAt = time.Now()
}

func (w *Workspace) UpdatedAt() time.Time {
	return w.updatedAt
}

func (w *Workspace) StripeCustomerName() string {
	return fmt.Sprintf("workspace:%s_%s", w.id, w.alias)
}
