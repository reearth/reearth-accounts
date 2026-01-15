package workspace

import (
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
	updatedAt time.Time
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

func (w *Workspace) UpdatedAt() time.Time {
	return w.updatedAt
}
