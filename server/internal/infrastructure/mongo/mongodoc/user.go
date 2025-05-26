package mongodoc

import (
	"time"

	"github.com/reearth/reearth-accounts/pkg/user"
	"github.com/reearth/reearthx/account/accountdomain"
	acUser "github.com/reearth/reearthx/account/accountdomain/user"
	"github.com/reearth/reearthx/mongox"
)

type UserDocument struct {
	ID            string                 `bson:"id"`
	Name          string                 `bson:"name"`
	Alias         string                 `bson:"alias"`
	Email         string                 `bson:"email"`
	Subs          []string               `bson:"subs"`
	Workspace     string                 `bson:"workspace"`
	Team          string                 `bson:",omitempty"`
	Lang          string                 `bson:"lang"`
	Theme         string                 `bson:"theme"`
	Password      []byte                 `bson:"password"`
	PasswordReset *PasswordResetDocument `bson:"passwordReset,omitempty"`
	Verification  *UserVerificationDoc
	Metadata      *UserMetadataDoc
}

type UserVerificationDoc struct {
	Code       string
	Expiration time.Time
	Verified   bool
}

type UserMetadataDoc struct {
	PhotoURL    string
	Description string
	Website     string
}

type PasswordResetDocument struct {
	Token     string
	CreatedAt time.Time
}

type UserConsumer = mongox.SliceFuncConsumer[*UserDocument, *user.User]

func NewUserConsumer() *UserConsumer {
	return mongox.NewSliceFuncConsumer(func(d *UserDocument) (*user.User, error) {
		m, err := d.Model()
		if err != nil {
			return nil, err
		}
		return m, nil
	})
}

func NewUser(usr user.User) (*UserDocument, string) {
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
		}
	}
	return &UserDocument{
		ID:            id,
		Name:          usr.Name(),
		Alias:         usr.Alias(),
		Email:         usr.Email(),
		Subs:          authsdoc,
		Workspace:     usr.Workspace().String(),
		Lang:          usr.Lang().String(),
		Theme:         string(usr.Theme()),
		Password:      usr.Password(),
		PasswordReset: pwdResetDoc,
		Verification:  verificationDoc,
		Metadata:      metadataDoc,
	}, id
}

func (d *UserDocument) Model() (*user.User, error) {
	uid, err := user.IDFrom(d.ID)
	if err != nil {
		return nil, err
	}

	wid := d.Workspace
	if wid == "" {
		wid = d.Team
	}

	tid, err := accountdomain.WorkspaceIDFrom(wid)
	if err != nil {
		return nil, err
	}

	auths := make([]acUser.Auth, 0, len(d.Subs))
	for _, sub := range d.Subs {
		auths = append(auths, acUser.Auth{
			Sub: sub,
		})
	}

	var v *acUser.Verification
	if d.Verification != nil {
		v = acUser.VerificationFrom(
			d.Verification.Code,
			d.Verification.Expiration,
			d.Verification.Verified,
		)
	}

	var metadata *user.Metadata
	if d.Metadata != nil {
		metadata = user.MetadataFrom(d.Metadata.PhotoURL, d.Metadata.Description, d.Metadata.Website)
	}

	usr, err := user.New().
		ID(uid).
		Name(d.Name).
		Email(d.Email).
		Metadata(metadata).
		Alias(d.Alias).
		Auths(auths).
		Workspace(tid).
		LangFrom(d.Lang).
		Verification(v).
		EncodedPassword(d.Password).
		PasswordReset(d.PasswordReset.Model()).
		Theme(acUser.Theme(d.Theme)).
		Build()
	if err != nil {
		return nil, err
	}

	return usr, nil
}

func (d *PasswordResetDocument) Model() *acUser.PasswordReset {
	if d == nil {
		return nil
	}
	return &acUser.PasswordReset{
		Token:     d.Token,
		CreatedAt: d.CreatedAt,
	}
}
