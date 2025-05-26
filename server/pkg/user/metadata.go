package user

type Metadata struct {
	photoURL    string
	description string
	website     string
}

func NewMetadata() *Metadata {
	return &Metadata{}
}

func MetadataFrom(photoURL, description, website string) *Metadata {
	return &Metadata{
		photoURL:    photoURL,
		description: description,
		website:     website,
	}
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
