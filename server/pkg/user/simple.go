package user

type Simple struct {
	ID    ID
	Name  string
	Email string
	Host  string
}

func SimpleFrom(u *User) *Simple {
	if u == nil {
		return nil
	}
	return &Simple{
		ID:    u.ID(),
		Name:  u.Name(),
		Email: u.Email(),
		Host:  u.Host(),
	}
}

type SimpleList []*Simple
