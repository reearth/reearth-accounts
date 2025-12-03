package mongodoc

import (
	"time"

	"github.com/labstack/gommon/log"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/mongox"
)

type PasswordResetDocument struct {
	Token     string    `json:"token" jsonschema:"description=Password reset token. Default: \"\""`
	CreatedAt time.Time `json:"createdat" jsonschema:"description=Token creation timestamp"`
}

type UserDocument struct {
	ID            string                 `json:"id" jsonschema:"description=User ID (ULID format)"`
	Name          string                 `json:"name" jsonschema:"description=User display name"`
	Alias         string                 `json:"alias" jsonschema:"description=Unique user handle/alias. Default: \"\""`
	Email         string                 `json:"email" jsonschema:"description=User email address"`
	Subs          []string               `json:"subs" jsonschema:"description=OAuth subject identifiers for authentication providers. Default: []"`
	Workspace     string                 `json:"workspace" jsonschema:"description=Personal workspace ID (ULID format)"`
	Team          string                 `json:"team" bson:",omitempty" jsonschema:"description=Legacy team field (deprecated, use workspace)"`
	Lang          string                 `json:"lang" jsonschema:"description=User language preference. Default: \"\" (deprecated, move to metadata)"`
	Theme         string                 `json:"theme" jsonschema:"description=User UI theme preference. Default: \"\" (deprecated, move to metadata)"`
	Password      []byte                 `json:"password" jsonschema:"description=Hashed password (bcrypt)"`
	PasswordReset *PasswordResetDocument `json:"passwordreset" jsonschema:"description=Password reset token information"`
	Verification  *UserVerificationDoc   `json:"verification" jsonschema:"description=Email verification state. Default: null"`
	Metadata      UserMetadataDoc        `json:"metadata" jsonschema:"description=Extended user metadata. Default: {}"`
}

type UserVerificationDoc struct {
	Code       string    `json:"code" jsonschema:"description=Verification code. Default: \"\""`
	Expiration time.Time `json:"expiration" jsonschema:"description=Verification code expiration timestamp"`
	Verified   bool      `json:"verified" jsonschema:"description=Whether the email has been verified. Default: false"`
}

type UserMetadataDoc struct {
	Description string `json:"description" jsonschema:"description=User bio/description. Default: \"\""`
	Website     string `json:"website" jsonschema:"description=User website URL. Default: \"\""`
	PhotoURL    string `json:"photourl" jsonschema:"description=Profile photo URL. Default: \"\""`
	Lang        string `json:"lang" jsonschema:"description=Language metadata. Default: \"\""`
	Theme       string `json:"theme" jsonschema:"description=Theme metadata. Default: \"\""`
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
		log.Warn("error converting user id: ", err)
		log.Error("user id: ", d.ID)
		return nil, err
	}

	wid := d.Workspace
	if wid == "" {
		wid = d.Team
	}

	tid, err := id.WorkspaceIDFrom(wid)
	if err != nil {
		log.Warn("error converting workspace id: ", err)
		log.Warn("user id: ", d.ID)
		log.Error("workspace id: ", wid)
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
