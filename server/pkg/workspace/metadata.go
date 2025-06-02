package workspace

type Metadata struct {
	description  string
	website      string
	location     string
	billingEmail string
	photoURL     string
}

func NewMetadata() *Metadata {
	return &Metadata{}
}

func MetadataFrom(description, website, location, billingEmail, photoURL string) *Metadata {
	return &Metadata{
		description:  description,
		website:      website,
		location:     location,
		billingEmail: billingEmail,
		photoURL:     photoURL,
	}
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

func (m *Metadata) Location() string {
	return m.location
}

func (m *Metadata) SetLocation(location string) {
	m.location = location
}

func (m *Metadata) BillingEmail() string {
	return m.billingEmail
}

func (m *Metadata) SetBillingEmail(billingEmail string) {
	m.billingEmail = billingEmail
}

func (m *Metadata) PhotoURL() string {
	return m.photoURL
}

func (m *Metadata) SetPhotoURL(photoURL string) {
	m.photoURL = photoURL
}
