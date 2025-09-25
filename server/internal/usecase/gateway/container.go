package gateway

import "github.com/reearth/reearthx/mailer"

type Container struct {
	Authenticator Authenticator
	Mailer        mailer.Mailer
	Storage       Storage
}
