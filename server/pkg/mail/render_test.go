package mail

import (
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata")

var samples = map[Kind]Data{
	KindVerification: VerificationData{
		UserName:  "Alice",
		ActionURL: "https://auth.example.com/?user-verification-token=CODE",
	},
	KindPasswordReset: PasswordResetData{
		UserName:  "Alice",
		ActionURL: "https://auth.example.com/?pwd-reset-token=TOKEN",
	},
	KindVerificationResend: VerificationResendData{
		ActionURL: "https://verify.example.com/?mode=verifyEmail&oobCode=CODE",
	},
	KindPolicyNotice: PolicyNoticeData{
		ActionURL:     "https://policies.example.com/tos/2.0.0",
		EffectiveDate: time.Date(2027, 1, 1, 9, 0, 0, 0, time.FixedZone("JST", 9*3600)),
		Major:         true,
		PolicyTitle:   "Terms of Service",
		Version:       "2.0.0",
	},
}

func TestEveryKindHasEveryLang(t *testing.T) {
	for _, k := range Kinds {
		for _, l := range Langs {
			for _, name := range append([]string{textFile(k, l)}, htmlFiles(k, l)...) {
				_, err := fs.Stat(templateFS, name)
				assert.NoError(t, err, "kind %s lang %s", k, l)
			}

			d, ok := samples[k]
			require.True(t, ok, "no sample data for kind %s", k)
			m, err := Render(language.Make(string(l)), d)
			require.NoError(t, err, "kind %s lang %s", k, l)
			assert.Equal(t, k, m.Kind())
			assert.NotEmpty(t, m.Subject(), "kind %s lang %s", k, l)
			assert.NotEmpty(t, m.Text(), "kind %s lang %s", k, l)
			assert.NotEmpty(t, m.HTML(), "kind %s lang %s", k, l)
		}
	}
}

func TestNoOrphanTemplates(t *testing.T) {
	known := map[string]bool{}
	for _, k := range Kinds {
		for _, l := range Langs {
			known[textFile(k, l)] = true
			for _, name := range htmlFiles(k, l) {
				known[name] = true
			}
		}
	}

	names, err := fs.Glob(templateFS, "templates/*")
	require.NoError(t, err)
	for _, name := range names {
		assert.True(t, known[name], "%s belongs to no kind and lang in Kinds x Langs", name)
	}
}

func TestLangOf(t *testing.T) {
	tests := []struct {
		tag  language.Tag
		want Lang
	}{
		{language.Japanese, LangJA},
		{language.MustParse("ja-JP"), LangJA},
		{language.English, LangEN},
		{language.MustParse("en-GB"), LangEN},
		{language.Und, LangEN},
		{language.Tag{}, LangEN},
		{language.MustParse("und-JP"), LangEN},
		{language.French, LangEN},
		{language.MustParse("zh-Hant"), LangEN},
	}
	for _, tt := range tests {
		t.Run(tt.tag.String(), func(t *testing.T) {
			assert.Equal(t, tt.want, LangOf(tt.tag))
		})
	}
}

func TestRender_Golden(t *testing.T) {
	for _, k := range Kinds {
		for _, l := range Langs {
			t.Run(string(k)+"."+string(l), func(t *testing.T) {
				m, err := Render(language.Make(string(l)), samples[k])
				require.NoError(t, err)

				base := filepath.Join("testdata", string(k)+"."+string(l))
				golden(t, base+".txt", "Subject: "+m.Subject()+"\n\n"+m.Text())
				golden(t, base+".html", m.HTML())
			})
		}
	}
}

func golden(t *testing.T, path, got string) {
	t.Helper()
	if *update {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
		return
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err, "run go test ./pkg/mail -update to create it")
	assert.Equal(t, string(want), got)
}

func TestRender_UnsetLangIsEnglish(t *testing.T) {
	en, err := Render(language.English, samples[KindVerification])
	require.NoError(t, err)
	for _, tag := range []language.Tag{language.Und, {}, language.French} {
		m, err := Render(tag, samples[KindVerification])
		require.NoError(t, err)
		assert.Equal(t, en, m, tag.String())
	}
}

func TestRender_EscapesUserNameInHTML(t *testing.T) {
	name := `<script>alert("x")</script>`
	for _, l := range Langs {
		for _, d := range []Data{
			VerificationData{UserName: name, ActionURL: "https://auth.example.com/"},
			PasswordResetData{UserName: name, ActionURL: "https://auth.example.com/"},
		} {
			m, err := Render(language.Make(string(l)), d)
			require.NoError(t, err)
			assert.NotContains(t, m.HTML(), "<script>", "%s %s", d.Kind(), l)
			assert.Contains(t, m.HTML(), `&lt;script&gt;alert(&#34;x&#34;)&lt;/script&gt;`, "%s %s", d.Kind(), l)
			assert.Contains(t, m.Text(), name, "the text part is not HTML")
		}
	}
}

func TestRender_HTMLEscapesActionURL(t *testing.T) {
	m, err := Render(language.English, VerificationResendData{ActionURL: `https://verify.example.com/?token=a"b&c<d`})
	require.NoError(t, err)
	assert.Contains(t, m.HTML(), `href="https://verify.example.com/?token=a%22b&amp;c%3cd"`)
	assert.NotRegexp(t, regexp.MustCompile(`a"b|c<d`), m.HTML())
	assert.Contains(t, m.Text(), `https://verify.example.com/?token=a"b&c<d`)
}

type unknownData struct{}

func (unknownData) Kind() Kind { return "unknown" }

func TestRender_UnknownKind(t *testing.T) {
	_, err := Render(language.English, unknownData{})
	assert.Error(t, err)
}

func BenchmarkRender_Verification(b *testing.B) {
	for b.Loop() {
		if _, err := Render(language.Japanese, samples[KindVerification]); err != nil {
			b.Fatal(err)
		}
	}
}

func TestRender_PolicyNotice(t *testing.T) {
	minor := PolicyNoticeData{
		EffectiveDate: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
		PolicyTitle:   "Privacy Policy",
		Version:       "1.3.0",
	}
	for _, l := range Langs {
		t.Run(string(l), func(t *testing.T) {
			m, err := Render(language.Make(string(l)), samples[KindPolicyNotice])
			require.NoError(t, err)
			assert.Contains(t, m.Text(), "2027", "the date is in UTC")
			assert.Contains(t, m.Text(), "00:00", "09:00 JST is 00:00 UTC")
			assert.Contains(t, m.Text(), "https://policies.example.com/tos/2.0.0")
			assert.Contains(t, m.HTML(), `href="https://policies.example.com/tos/2.0.0"`)

			m, err = Render(language.Make(string(l)), minor)
			require.NoError(t, err)
			assert.Contains(t, m.Subject(), "Privacy Policy")
			assert.Contains(t, m.Text(), "1.3.0")
			assert.NotContains(t, m.Text(), "http", "no link without a URL")
			assert.NotContains(t, m.HTML(), `class="btn"`, "no button without a URL")
			golden(t, filepath.Join("testdata", "policy_notice_minor."+string(l)+".txt"), "Subject: "+m.Subject()+"\n\n"+m.Text())
		})
	}
}
