package role

import "errors"

var (
	ErrEmptyName = errors.New("role name can't be empty")
)

type Role struct {
	id   ID
	name string
}

func (r *Role) ID() ID {
	if r == nil {
		return ID{}
	}
	return r.id
}

func (r *Role) Name() string {
	if r == nil {
		return ""
	}
	return r.name
}

func (r *Role) Rename(name string) {
	if r == nil {
		return
	}
	r.name = name
}
