package mongodoc

import (
	"time"

	"github.com/reearth/reearth-accounts/pkg/id"
	"github.com/reearth/reearth-accounts/pkg/user"
	"github.com/reearth/reearthx/mongox"
)

type UserDocument struct {
	Alias         string `bson:"alias"`
	Email         string `bson:"email"`
	ID            string `bson:"id"`
	Lang          string `bson:"lang"`
	Metadata      *UserMetadataDoc
	Name          string                 `bson:"name"`
	Password      []byte                 `bson:"password"`
	PasswordReset *PasswordResetDocument `bson:"passwordReset,omitempty"`
	Subs          []string               `bson:"subs"`
	Team          string                 `bson:",omitempty"`
	Theme         string                 `bson:"theme"`
	Verification  *UserVerificationDoc
	Workspace     string `bson:"workspace"`
}

type UserVerificationDoc struct {
	Code       string
	Expiration time.Time
	Verified   bool
}

type UserMetadataDoc struct {
	Description string
	Lang        string
	PhotoURL    string
	Theme       string
	Website     string
}

type PasswordResetDocument struct {
	CreatedAt time.Time
	Token     string
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

func NewUser(usr *user.User) (*UserDocument, string) {
	id := usr.ID().String()

	auths := usr.Auths()
	authsdoc := make([]string, 0, len(auths))
	for _, a := range auths {
		authsdoc = append(authsdoc, a.Sub)
	}

	var verificationDoc *UserVerificationDoc
	if usr.Verification() != nil {
		verificationDoc = &UserVerificationDoc{
			Code:       usr.Verification().Code(),
			Expiration: usr.Verification().Expiration(),
			Verified:   usr.Verification().IsVerified(),
		}
	}

	pwdReset := usr.PasswordReset()
	var pwdResetDoc *PasswordResetDocument
	if pwdReset != nil {
		pwdResetDoc = &PasswordResetDocument{
			Token:     pwdReset.Token,
			CreatedAt: pwdReset.CreatedAt,
		}
	}

	metadataDoc := &UserMetadataDoc{}
	if usr.Metadata() != nil {
		metadataDoc = &UserMetadataDoc{
			PhotoURL:    usr.Metadata().PhotoURL(),
			Description: usr.Metadata().Description(),
			Website:     usr.Metadata().Website(),
			Lang:        usr.Metadata().Lang().String(),
			Theme:       string(usr.Metadata().Theme()),
		}
	}
	return &UserDocument{
		ID:            id,
		Name:          usr.Name(),
		Alias:         usr.Alias(),
		Email:         usr.Email(),
		Subs:          authsdoc,
		Workspace:     usr.Workspace().String(),
		Password:      usr.Password(),
		PasswordReset: pwdResetDoc,
		Verification:  verificationDoc,
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
		v = user.VerificationFrom(
			d.Verification.Code,
			d.Verification.Expiration,
			d.Verification.Verified,
		)
	}

	var metadata *user.Metadata
	if d.Metadata != nil {
		metadata = user.NewMetadata()
		metadata.SetDescription(d.Metadata.Description)
		metadata.SetWebsite(d.Metadata.Website)
		metadata.SetPhotoURL(d.Metadata.PhotoURL)
		metadata.LangFrom(d.Metadata.Lang)
		metadata.SetTheme(user.Theme(d.Metadata.Theme))
	}

	usr, err := user.New().
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

	return usr, nil
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
