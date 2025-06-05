package user

import "golang.org/x/text/language"

type Metadata struct {
	photoURL    string
	description string
	website     string
	lang        language.Tag
	theme       Theme
}

func NewMetadata() *Metadata {
	return &Metadata{}
}

func MetadataFrom(photoURL, description, website string, lang language.Tag, theme Theme) *Metadata {
	return &Metadata{
		photoURL:    photoURL,
		description: description,
		website:     website,
		lang:        lang,
		theme:       theme,
	}
}

func (m *Metadata) LangFrom(lang string) *Metadata {
	if lang == "" {
		m.lang = language.Und
	} else if l, err := language.Parse(lang); err == nil {
		m.lang = l
	}
	return m
}

func (m *Metadata) PhotoURL() string {
	return m.photoURL
}

func (m *Metadata) SetPhotoURL(url string) {
	m.photoURL = url
}

func (m *Metadata) Description() string {
	return m.description
}

func (m *Metadata) SetDescription(description string) {
	m.description = description
}

func (m *Metadata) Website() string {
	return m.website
}

func (m *Metadata) SetWebsite(website string) {
	m.website = website
}

func (m *Metadata) Lang() language.Tag {
	return m.lang
}

func (m *Metadata) SetLang(lang language.Tag) {
	m.lang = lang
}

func (m *Metadata) Theme() Theme {
	if !m.theme.Valid() {
		return ThemeDefault
	}
	return m.theme
}

func (m *Metadata) SetTheme(theme Theme) {
	m.theme = theme
}
