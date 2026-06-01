package pgdoc

import (
	"encoding/json"
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/user"
)

type UserMetadataJSON struct {
	Description string `json:"description"`
	Website     string `json:"website"`
	PhotoURL    string `json:"photourl"`
	Lang        string `json:"lang"`
	Theme       string `json:"theme"`
}

type UserVerificationJSON struct {
	Code       string    `json:"code"`
	Expiration time.Time `json:"expiration"`
	Verified   bool      `json:"verified"`
}

type UserPasswordResetJSON struct {
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"createdat"`
}

type UserRow struct {
	ID             string
	Name           string
	Alias          string
	Email          string
	Workspace      string
	Password       []byte
	Subs           []string
	LatestLogoutAt *time.Time
	Metadata       []byte // jsonb
	Verification   []byte // jsonb (nullable)
	PasswordReset  []byte // jsonb (nullable)
	Team           *string
	Lang           *string
	Theme          *string
	UpdatedAt      time.Time
}

func NewUserRow(u *user.User) *UserRow {
	auths := u.Auths()
	subs := make([]string, 0, len(auths))
	for _, a := range auths {
		subs = append(subs, a.Sub)
	}

	var meta []byte
	if m := u.Metadata(); m != nil {
		meta, _ = json.Marshal(UserMetadataJSON{
			Description: m.Description(),
			Website:     m.Website(),
			PhotoURL:    m.PhotoURL(),
			Lang:        m.Lang().String(),
			Theme:       string(m.Theme()),
		})
	} else {
		meta = []byte("{}")
	}

	var verification []byte
	if v := u.Verification(); v != nil {
		verification, _ = json.Marshal(UserVerificationJSON{
			Code:       v.Code(),
			Expiration: v.Expiration(),
			Verified:   v.IsVerified(),
		})
	}

	var pwReset []byte
	if pr := u.PasswordReset(); pr != nil {
		pwReset, _ = json.Marshal(UserPasswordResetJSON{Token: pr.Token, CreatedAt: pr.CreatedAt})
	}

	var llat *time.Time
	if t := u.LatestLogoutAt(); !t.IsZero() {
		tt := t
		llat = &tt
	}

	updatedAt := u.UpdatedAt()
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}

	return &UserRow{
		ID:             u.ID().String(),
		Name:           u.Name(),
		Alias:          u.Alias(),
		Email:          u.Email(),
		Workspace:      u.Workspace().String(),
		Password:       u.Password(),
		Subs:           subs,
		LatestLogoutAt: llat,
		Metadata:       meta,
		Verification:   verification,
		PasswordReset:  pwReset,
		UpdatedAt:      updatedAt,
	}
}

func (r *UserRow) Model() (*user.User, error) {
	uid, err := id.UserIDFrom(r.ID)
	if err != nil {
		return nil, err
	}
	wid := r.Workspace
	if wid == "" && r.Team != nil {
		wid = *r.Team
	}
	tid, err := id.WorkspaceIDFrom(wid)
	if err != nil {
		return nil, err
	}

	auths := make([]user.Auth, 0, len(r.Subs))
	for _, s := range r.Subs {
		auths = append(auths, user.AuthFrom(s))
	}

	var v *user.Verification
	if len(r.Verification) > 0 {
		var vj UserVerificationJSON
		if err := json.Unmarshal(r.Verification, &vj); err != nil {
			return nil, err
		}
		v = user.VerificationFrom(vj.Code, vj.Expiration, vj.Verified)
	}

	var pwReset *user.PasswordReset
	if len(r.PasswordReset) > 0 {
		var pj UserPasswordResetJSON
		if err := json.Unmarshal(r.PasswordReset, &pj); err != nil {
			return nil, err
		}
		pwReset = &user.PasswordReset{Token: pj.Token, CreatedAt: pj.CreatedAt}
	}

	var mj UserMetadataJSON
	if len(r.Metadata) > 0 {
		if err := json.Unmarshal(r.Metadata, &mj); err != nil {
			return nil, err
		}
	}
	metadata := user.NewMetadata()
	metadata.SetDescription(mj.Description)
	metadata.SetWebsite(mj.Website)
	metadata.SetPhotoURL(mj.PhotoURL)
	metadata.LangFrom(mj.Lang)
	metadata.SetTheme(user.Theme(mj.Theme))

	var llat time.Time
	if r.LatestLogoutAt != nil {
		llat = *r.LatestLogoutAt
	}

	return user.New().
		ID(uid).
		Name(r.Name).
		Email(r.Email).
		LatestLogoutAt(llat).
		Metadata(metadata).
		Alias(r.Alias).
		Auths(auths).
		Workspace(tid).
		Verification(v).
		EncodedPassword(r.Password).
		PasswordReset(pwReset).
		UpdatedAt(r.UpdatedAt).
		Build()
}
