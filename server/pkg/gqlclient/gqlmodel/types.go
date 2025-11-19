package gqlmodel

type Lang string

func (*Lang) GetGraphQLType() string { return "Lang" }

type Theme string

func (*Theme) GetGraphQLType() string { return "Theme" }

func NewLang(s string) *Lang {
	l := Lang(s)
	return &l
}

func NewTheme(s string) *Theme {
	t := Theme(s)
	return &t
}
