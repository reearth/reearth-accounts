package workspace

import (
	"fmt"
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/sso"
	"github.com/reearth/reearthx/util"
)

type Workspace struct {
	id        ID
	name      string
	alias     string
	email     string
	members   *Members
	metadata  Metadata
	policy    *PolicyID
	ssoConfig *sso.Config
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

func (w *Workspace) IsEnterprise() bool {
	return w.policy != nil && *w.policy == PolicyEnterprise
}

func (w *Workspace) IsPersonal() bool {
	return w.members.Fixed()
}

func (w *Workspace) SSOConfig() *sso.Config {
	return w.ssoConfig
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

func (w *Workspace) DeleteSSOConfig() {
	w.ssoConfig = nil
	w.updatedAt = time.Now()
}

func (w *Workspace) SetPolicy(policy *PolicyID) {
	w.policy = util.CloneRef(policy)
	w.updatedAt = time.Now()
}

func (w *Workspace) SetSSOConfig(cfg *sso.Config) {
	w.ssoConfig = cfg
	w.updatedAt = time.Now()
}

func (w *Workspace) DeleteIntegrations(iids IntegrationIDList) error {
	err := w.members.DeleteIntegrations(iids)
	if err != nil {
		return err
	}
	w.updatedAt = time.Now()
	return nil
}

func (w *Workspace) UpdatedAt() time.Time {
	return w.updatedAt
}

func (w *Workspace) StripeCustomerName() string {
	return fmt.Sprintf("workspace:%s_%s", w.id, w.alias)
}
