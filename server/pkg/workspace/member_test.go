package workspace

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMetadata(t *testing.T) {
	metadata := NewMetadata()
	assert.Equal(t, &Metadata{}, metadata)
}

func TestMetadataFrom(t *testing.T) {
	metadata := MetadataFrom("description", "website", "location", "billingEmail", "photo url")
	assert.Equal(t, "description", metadata.Description())
	assert.Equal(t, "website", metadata.Website())
	assert.Equal(t, "location", metadata.Location())
	assert.Equal(t, "billingEmail", metadata.BillingEmail())
}

func TestMetadata_Description(t *testing.T) {
	metadata := &Metadata{description: "description"}
	assert.Equal(t, "description", metadata.Description())
}

func TestMetadata_Website(t *testing.T) {
	metadata := &Metadata{website: "website"}
	assert.Equal(t, "website", metadata.Website())
}

func TestMetadata_SetDescription(t *testing.T) {
	metadata := &Metadata{}
	metadata.SetDescription("new description")
	assert.Equal(t, "new description", metadata.Description())
}

func TestMetadata_SetWebsite(t *testing.T) {
	metadata := &Metadata{}
	metadata.SetWebsite("new website")
	assert.Equal(t, "new website", metadata.Website())
}

func TestMetadata_Location(t *testing.T) {
	metadata := &Metadata{location: "location"}
	assert.Equal(t, "location", metadata.Location())
}

func TestMetadata_SetLocation(t *testing.T) {
	metadata := &Metadata{}
	metadata.SetLocation("new location")
	assert.Equal(t, "new location", metadata.Location())
}

func TestMetadata_BillingEmail(t *testing.T) {
	metadata := &Metadata{billingEmail: "billingEmail"}
	assert.Equal(t, "billingEmail", metadata.BillingEmail())
}

func TestMetadata_SetBillingEmail(t *testing.T) {
	metadata := &Metadata{}
	metadata.SetBillingEmail("new billing email")
	assert.Equal(t, "new billing email", metadata.BillingEmail())
}

func TestMetadata_PhotoURL(t *testing.T) {
	metadata := &Metadata{photoURL: "photo url"}
	assert.Equal(t, "photo url", metadata.PhotoURL())
}

func TestMetadata_SetPhotoURL(t *testing.T) {
	metadata := &Metadata{}
	metadata.SetPhotoURL("new photo url")
	assert.Equal(t, "new photo url", metadata.PhotoURL())
}
