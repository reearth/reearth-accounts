package workspace

import "github.com/reearth/reearthx/util"

type Workspace struct {
	id          ID
	name        string
	displayName string
	members     *Members
	policy      *PolicyID
	location    string
}

func (t *Workspace) ID() ID {
	return t.id
}

func (t *Workspace) Name() string {
	return t.name
}

func (t *Workspace) DisplayName() string {
	return t.displayName
}

func (t *Workspace) Members() *Members {
	return t.members
}

func (t *Workspace) IsPersonal() bool {
	return t.members.Fixed()
}

func (t *Workspace) Location() string {
	return t.location
}

func (t *Workspace) LocationOr(def string) string {
	if t.location == "" {
		return def
	}
	return t.location
}

func (t *Workspace) Rename(name string) {
	t.name = name
}

func (t *Workspace) UpdateDisplayName(displayName string) {
	t.displayName = displayName
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
}
