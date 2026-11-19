package mail

import (
	"bytes"
	"embed"
	"fmt"
	htmltemplate "html/template"
	texttemplate "text/template"
	"time"

	"golang.org/x/text/language"
)

// Kinds lists every mail accounts sends.
var Kinds = []Kind{KindVerification, KindPasswordReset, KindVerificationResend, KindPolicyNotice}

type Lang string

const (
	LangEN Lang = "en"
	LangJA Lang = "ja"
)

var Langs = []Lang{LangEN, LangJA}

var baseLangs = func() map[language.Base]Lang {
	m := make(map[language.Base]Lang, len(Langs))
	for _, l := range Langs {
		m[language.MustParseBase(string(l))] = l
	}
	return m
}()

// LangOf picks the template language by the tag's base language and falls
// back to English. An und or zero tag only has a guessed base, so it never
// matches.
func LangOf(t language.Tag) Lang {
	base, conf := t.Base()
	if conf != language.Exact {
		return LangEN
	}
	if l, ok := baseLangs[base]; ok {
		return l
	}
	return LangEN
}

type Data interface{ Kind() Kind }

type VerificationData struct {
	UserName  string
	ActionURL string
}

type PasswordResetData struct {
	UserName  string
	ActionURL string
}

type VerificationResendData struct {
	ActionURL string
}

// PolicyNoticeData announces an upcoming policy version. ActionURL links the
// policy text in the user's language and may be empty.
type PolicyNoticeData struct {
	ActionURL     string
	EffectiveDate time.Time
	Major         bool
	PolicyTitle   string
	Version       string
}

func (VerificationData) Kind() Kind       { return KindVerification }
func (PasswordResetData) Kind() Kind      { return KindPasswordReset }
func (VerificationResendData) Kind() Kind { return KindVerificationResend }
func (PolicyNoticeData) Kind() Kind       { return KindPolicyNotice }

//go:embed templates/*.tmpl
var templateFS embed.FS

type kindTemplates struct {
	text *texttemplate.Template
	html *htmltemplate.Template
}

var registry = buildRegistry()

func buildRegistry() map[Kind]map[Lang]kindTemplates {
	r := make(map[Kind]map[Lang]kindTemplates, len(Kinds))
	for _, k := range Kinds {
		r[k] = make(map[Lang]kindTemplates, len(Langs))
		for _, l := range Langs {
			r[k][l] = kindTemplates{
				text: texttemplate.Must(texttemplate.New("").ParseFS(templateFS, textFile(k, l))),
				html: htmltemplate.Must(htmltemplate.New("").ParseFS(templateFS, htmlFiles(k, l)...)),
			}
		}
	}
	return r
}

func textFile(k Kind, l Lang) string {
	return fmt.Sprintf("templates/%s.%s.txt.tmpl", k, l)
}

func htmlFiles(k Kind, l Lang) []string {
	return []string{
		"templates/layout.html.tmpl",
		fmt.Sprintf("templates/layout.%s.html.tmpl", l),
		fmt.Sprintf("templates/%s.%s.html.tmpl", k, l),
	}
}

// Render renders d in the user's language.
func Render(lang language.Tag, d Data) (Message, error) {
	byLang, ok := registry[d.Kind()]
	if !ok {
		return Message{}, fmt.Errorf("mail: no templates for kind %q", d.Kind())
	}
	t := byLang[LangOf(lang)]

	var subject, text, html bytes.Buffer
	if err := t.text.ExecuteTemplate(&subject, "subject", d); err != nil {
		return Message{}, err
	}
	if err := t.text.ExecuteTemplate(&text, "text", d); err != nil {
		return Message{}, err
	}
	if err := t.html.ExecuteTemplate(&html, "html", d); err != nil {
		return Message{}, err
	}
	return Message{kind: d.Kind(), subject: subject.String(), text: text.String(), html: html.String()}, nil
}
