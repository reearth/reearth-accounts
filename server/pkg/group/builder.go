package group

type Builder struct {
	g *Group
}

func New() *Builder {
	return &Builder{g: &Group{}}
}

func (b *Builder) Build() (*Group, error) {
	if b.g.id.IsNil() {
		return nil, ErrInvalidID
	}
	if b.g.name == "" {
		return nil, ErrEmptyName
	}
	return b.g, nil
}

func (b *Builder) MustBuild() *Group {
	g, err := b.Build()
	if err != nil {
		panic(err)
	}
	return g
}

func (b *Builder) ID(id ID) *Builder {
	b.g.id = id
	return b
}

func (b *Builder) NewID() *Builder {
	b.g.id = NewID()
	return b
}

func (b *Builder) Name(name string) *Builder {
	b.g.name = name
	return b
}
