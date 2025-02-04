package group

import "errors"

var (
	ErrEmptyName = errors.New("group name can't be empty")
)

type Group struct {
	id   ID
	name string
}

func (g *Group) ID() ID {
	if g == nil {
		return ID{}
	}
	return g.id
}

func (g *Group) Name() string {
	if g == nil {
		return ""
	}
	return g.name
}

func (g *Group) Rename(name string) {
	if g == nil {
		return
	}
	g.name = name
}
