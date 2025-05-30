package user

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/text/language"
)

func TestNewMetadata(t *testing.T) {
	metadata := NewMetadata()
	assert.Equal(t, &Metadata{}, metadata)
}

func TestMetadata_MetadataFrom(t *testing.T) {
	lang := language.Make("en")
	metadata := MetadataFrom("photo url", "description", "website", lang, ThemeDark)
	assert.Equal(t, "photo url", metadata.PhotoURL())
	assert.Equal(t, "description", metadata.Description())
	assert.Equal(t, "website", metadata.Website())
	assert.Equal(t, lang, metadata.Lang())
	assert.Equal(t, ThemeDark, metadata.Theme())
}

func TestMetadataFrom(t *testing.T) {
	metadata := NewMetadata()
	metadata.LangFrom("en")
	metadata.SetTheme(ThemeDark)
	metadata.SetPhotoURL("photo url")
	metadata.SetDescription("description")
	metadata.SetWebsite("website")
	assert.Equal(t, "en", metadata.Lang().String())
	assert.Equal(t, ThemeDark, metadata.Theme())
	assert.Equal(t, "photo url", metadata.PhotoURL())
	assert.Equal(t, "description", metadata.Description())
	assert.Equal(t, "website", metadata.Website())
}

func TestMetadata_Description(t *testing.T) {
	metadata := &Metadata{description: "description"}
	assert.Equal(t, "description", metadata.Description())
}

func TestMetadata_Website(t *testing.T) {
	metadata := &Metadata{website: "website"}
	assert.Equal(t, "website", metadata.Website())
}

func TestMetadata_Lang(t *testing.T) {
	l := language.Make("en")
	metadata := &Metadata{lang: l}
	assert.Equal(t, "en", metadata.Lang().String())
}

func TestMetadata_PhotoURL(t *testing.T) {
	metadata := &Metadata{photoURL: "photo url"}
	assert.Equal(t, "photo url", metadata.PhotoURL())
}

func TestMetadata_Theme(t *testing.T) {
	metadata := &Metadata{theme: ThemeDark}
	assert.Equal(t, ThemeDark, metadata.Theme())
}
