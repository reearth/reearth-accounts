package role

type Builder struct {
	r *Role
}

func New() *Builder {
	return &Builder{r: &Role{}}
}

func (b *Builder) Build() (*Role, error) {
	if b.r.id.IsNil() {
		return nil, ErrInvalidID
	}
	if b.r.name == "" {
		return nil, ErrEmptyName
	}
	return b.r, nil
}

func (b *Builder) MustBuild() *Role {
	g, err := b.Build()
	if err != nil {
		panic(err)
	}
	return g
}

func (b *Builder) ID(id ID) *Builder {
	b.r.id = id
	return b
}

func (b *Builder) NewID() *Builder {
	b.r.id = NewID()
	return b
}

func (b *Builder) Name(name string) *Builder {
	b.r.name = name
	return b
}
