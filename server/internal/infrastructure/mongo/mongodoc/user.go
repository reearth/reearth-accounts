package mongodoc

import (
	"time"

	"github.com/reearth/reearth-accounts/pkg/id"
	"github.com/reearth/reearth-accounts/pkg/user"
	"github.com/reearth/reearthx/mongox"
)

type PasswordResetDocument struct {
	Token     string
	CreatedAt time.Time
}

type UserDocument struct {
	ID            string
	Name          string
	Alias         string
	Email         string
	Subs          []string
	Workspace     string
	Team          string `bson:",omitempty"`
	Lang          string
	Theme         string
	Password      []byte
	PasswordReset *PasswordResetDocument
	Verification  *UserVerificationDoc
	Metadata      UserMetadataDoc
}

type UserVerificationDoc struct {
	Code       string
	Expiration time.Time
	Verified   bool
}

type UserMetadataDoc struct {
	Description string
	Website     string
	PhotoURL    string
	Lang        string
	Theme       string
}

func NewUser(user *user.User) (*UserDocument, string) {
	id := user.ID().String()
	auths := user.Auths()
	authsdoc := make([]string, 0, len(auths))
	for _, a := range auths {
		authsdoc = append(authsdoc, a.Sub)
	}
	var v *UserVerificationDoc
	if user.Verification() != nil {
		v = &UserVerificationDoc{
			Code:       user.Verification().Code(),
			Expiration: user.Verification().Expiration(),
			Verified:   user.Verification().IsVerified(),
		}
	}
	pwdReset := user.PasswordReset()

	var pwdResetDoc *PasswordResetDocument
	if pwdReset != nil {
		pwdResetDoc = &PasswordResetDocument{
			Token:     pwdReset.Token,
			CreatedAt: pwdReset.CreatedAt,
		}
	}

	metadataDoc := UserMetadataDoc{
		Description: user.Metadata().Description(),
		Website:     user.Metadata().Website(),
		PhotoURL:    user.Metadata().PhotoURL(),
		Lang:        user.Metadata().Lang().String(),
		Theme:       string(user.Metadata().Theme()),
	}

	return &UserDocument{
		ID:            id,
		Name:          user.Name(),
		Alias:         user.Alias(),
		Email:         user.Email(),
		Subs:          authsdoc,
		Workspace:     user.Workspace().String(),
		Verification:  v,
		Password:      user.Password(),
		PasswordReset: pwdResetDoc,
		Metadata:      metadataDoc,
	}, id
}

func (d *UserDocument) Model() (*user.User, error) {
	uid, err := id.UserIDFrom(d.ID)
	if err != nil {
		return nil, err
	}

	wid := d.Workspace
	if wid == "" {
		wid = d.Team
	}

	tid, err := id.WorkspaceIDFrom(wid)
	if err != nil {
		return nil, err
	}

	auths := make([]user.Auth, 0, len(d.Subs))
	for _, s := range d.Subs {
		auths = append(auths, user.AuthFrom(s))
	}

	var v *user.Verification
	if d.Verification != nil {
		v = user.VerificationFrom(d.Verification.Code, d.Verification.Expiration, d.Verification.Verified)
	}

	metadata := user.NewMetadata()
	metadata.SetDescription(d.Metadata.Description)
	metadata.SetWebsite(d.Metadata.Website)
	metadata.SetPhotoURL(d.Metadata.PhotoURL)
	metadata.LangFrom(d.Metadata.Lang)
	metadata.SetTheme(user.Theme(d.Metadata.Theme))

	u, err := user.New().
		ID(uid).
		Name(d.Name).
		Email(d.Email).
		Metadata(metadata).
		Alias(d.Alias).
		Auths(auths).
		Workspace(tid).
		Verification(v).
		EncodedPassword(d.Password).
		PasswordReset(d.PasswordReset.Model()).
		Build()

	if err != nil {
		return nil, err
	}
	return u, nil
}

func (d *PasswordResetDocument) Model() *user.PasswordReset {
	if d == nil {
		return nil
	}
	return &user.PasswordReset{
		Token:     d.Token,
		CreatedAt: d.CreatedAt,
	}
}

type UserConsumer = mongox.SliceFuncConsumer[*UserDocument, *user.User]

func NewUserConsumer(host string) *UserConsumer {
	return mongox.NewSliceFuncConsumer(func(d *UserDocument) (*user.User, error) {
		m, err := d.Model()
		if err != nil {
			return nil, err
		}
		return m.WithHost(host), nil
	})
}
