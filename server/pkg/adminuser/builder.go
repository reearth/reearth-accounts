package adminuser

import "time"

type Builder struct {
	u *AdminUser
}

func New() *Builder {
	return &Builder{u: &AdminUser{}}
}

func (b *Builder) Build() (*AdminUser, error) {
	if b.u.id.IsNil() {
		return nil, ErrInvalidID
	}
	if b.u.name == "" {
		return nil, ErrEmptyName
	}
	email, err := normalizeAndValidateEmail(b.u.email)
	if err != nil {
		return nil, err
	}
	b.u.email = email

	// Default to pending when no status was explicitly set.
	if b.u.status == "" {
		b.u.status = StatusPending
	}
	if !b.u.status.Valid() {
		return nil, ErrInvalidStatus
	}

	// Set default updatedAt if not explicitly set.
	if b.u.updatedAt.IsZero() {
		b.u.updatedAt = time.Now()
	}

	// Keep approval metadata consistent: an approved user must have an
	// approvedAt. approvedBy may legitimately be empty (e.g. bootstrap-approved
	// users have no human approver).
	if b.u.status == StatusApproved && b.u.approvedAt.IsZero() {
		b.u.approvedAt = b.u.updatedAt
	}

	return b.u, nil
}

func (b *Builder) MustBuild() *AdminUser {
	u, err := b.Build()
	if err != nil {
		panic(err)
	}
	return u
}

func (b *Builder) ID(id ID) *Builder {
	b.u.id = id
	return b
}

func (b *Builder) NewID() *Builder {
	b.u.id = NewID()
	return b
}

func (b *Builder) ApprovedAt(approvedAt time.Time) *Builder {
	b.u.approvedAt = approvedAt
	return b
}

func (b *Builder) ApprovedBy(approvedBy ID) *Builder {
	b.u.approvedBy = approvedBy
	return b
}

func (b *Builder) Email(email string) *Builder {
	b.u.email = email
	return b
}

func (b *Builder) Name(name string) *Builder {
	b.u.name = name
	return b
}

func (b *Builder) PictureURL(pictureURL string) *Builder {
	b.u.pictureURL = pictureURL
	return b
}

func (b *Builder) Status(status Status) *Builder {
	b.u.status = status
	return b
}

func (b *Builder) UpdatedAt(updatedAt time.Time) *Builder {
	b.u.updatedAt = updatedAt
	return b
}
