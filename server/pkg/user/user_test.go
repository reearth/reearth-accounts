package user

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/text/language"
)

func TestUser(t *testing.T) {
	u := &User{
		id:        NewID(),
		workspace: NewWorkspaceID(),
		name:      "xxx",
		lang:      language.Make("en"),
		email:     "ff@xx.zz",
		auths: []Auth{{
			Provider: "aaa",
			Sub:      "sss",
		}},
		theme: ThemeDark,
	}

	assert.Equal(t, u.id, u.ID())
	assert.Equal(t, "xxx", u.Name())
	assert.Equal(t, "xxx", u.DisplayName())
	assert.Equal(t, u.workspace, u.Workspace())
	assert.Equal(t, Auths([]Auth{{
		Provider: "aaa",
		Sub:      "sss",
	}}), u.Auths())
	assert.Equal(t, "ff@xx.zz", u.Email())
	assert.Equal(t, language.Make("en"), u.Lang())
	assert.Equal(t, ThemeDark, u.Theme())

	u.UpdateName("a")
	assert.Equal(t, "a", u.name)
	assert.Equal(t, "", u.displayName)
	assert.Equal(t, "a", u.DisplayName())
	assert.ErrorContains(t, u.UpdateEmail("ab"), "invalid email")
	assert.NoError(t, u.UpdateEmail("a@example.com"))
	assert.Equal(t, "a@example.com", u.email)
	u.UpdateLang(language.Und)
	assert.Equal(t, language.Und, u.lang)
	u.UpdateTheme(ThemeLight)
	assert.Equal(t, ThemeLight, u.theme)
	u.UpdateDisplayName("displayName")
	assert.Equal(t, "displayName", u.displayName)

	wid := NewWorkspaceID()
	u.UpdateWorkspace(wid)
	assert.Equal(t, wid, u.Workspace())

	u2 := u.Clone()
	assert.Equal(t, u, u2)
	assert.NotSame(t, u, u2)
}

func TestUser_Auths(t *testing.T) {
	u := &User{}

	assert.True(t, u.AddAuth(Auth{Provider: "xxx", Sub: "zzz"}))
	assert.Equal(t, &User{auths: []Auth{{Provider: "xxx", Sub: "zzz"}}}, u)
	assert.Equal(t, Auths([]Auth{{Provider: "xxx", Sub: "zzz"}}), u.Auths())
	assert.Equal(t, &Auth{Provider: "xxx", Sub: "zzz"}, u.GetAuthByProvider("xxx"))
	assert.Nil(t, u.GetAuthByProvider("xx"))

	assert.False(t, u.AddAuth(Auth{Provider: "xxx", Sub: "yyy"}))
	assert.Equal(t, &User{auths: []Auth{{Provider: "xxx", Sub: "zzz"}}}, u)

	assert.False(t, u.RemoveAuth(Auth{Provider: "xxx", Sub: "yyy"}))
	assert.Equal(t, &User{auths: []Auth{{Provider: "xxx", Sub: "zzz"}}}, u)
	assert.True(t, u.RemoveAuth(Auth{Provider: "xxx", Sub: "zzz"}))
	assert.Equal(t, &User{auths: []Auth{}}, u)

	assert.True(t, u.AddAuth(Auth{Provider: "xxx", Sub: "zzz"}))
	assert.False(t, u.RemoveAuthByProvider("yyy"))
	assert.True(t, u.RemoveAuthByProvider("xxx"))
	assert.Equal(t, &User{auths: []Auth{}}, u)

	u.ClearAuths()
	assert.Equal(t, &User{}, u)
}

func TestUser_Password(t *testing.T) {
	// bcrypt is not suitable for unit tests as it requires heavy computation
	DefaultPasswordEncoder = &NoopPasswordEncoder{}

	u := &User{}

	// empty
	assert.Nil(t, u.password)
	ok, err := u.MatchPassword("")
	assert.NoError(t, err)
	assert.False(t, ok)

	// ok
	assert.NoError(t, u.SetPassword("abcDEF0!"))
	assert.Equal(t, MustEncodedPassword("abcDEF0!"), u.password)
	ok, err = u.MatchPassword("abcDEF0!")
	assert.NoError(t, err)
	assert.True(t, ok)
	ok, err = u.MatchPassword("")
	assert.NoError(t, err)
	assert.False(t, ok)

	// non-latin characters password
	assert.NoError(t, u.SetPassword("Àêîôûtest1"))
	assert.Equal(t, MustEncodedPassword("Àêîôûtest1"), u.password)
	ok, err = u.MatchPassword("Àêîôûtest1")
	assert.NoError(t, err)
	assert.True(t, ok)
	ok, err = u.MatchPassword("Àêîôûtest")
	assert.NoError(t, err)
	assert.False(t, ok)

	// invalid password
	u.password = nil
	assert.Equal(t, ErrPasswordLength, u.SetPassword(""))
	assert.Nil(t, u.password)
}

func TestUser_PasswordReset(t *testing.T) {
	u := &User{}
	u.SetPasswordReset(&PasswordReset{
		Token:     "xzy",
		CreatedAt: time.Unix(0, 0),
	})
	assert.Equal(t, &PasswordReset{
		Token:     "xzy",
		CreatedAt: time.Unix(0, 0),
	}, u.PasswordReset())
}

func TestUser_Verification(t *testing.T) {
	v := NewVerification()
	u := &User{}
	u.SetVerification(v)
	assert.Equal(t, v, u.Verification())
}
