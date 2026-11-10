package adapter

import (
	"context"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type authTimeClaims struct {
	AuthTime *jwt.NumericDate `json:"auth_time"`
	jwt.RegisteredClaims
}

// AuthTime returns the auth_time claim of the request's token, or nil when
// there is no token or no readable claim. The claims are read without
// verifying the signature. In production the JWT middleware has verified the
// token before this runs. In debug mode the X-Reearth-Debug-Auth-Token header
// can supply any token, but debug headers already allow impersonating any
// user, so debug mode is not a security boundary.
func AuthTime(ctx context.Context) *time.Time {
	ai := GetAuthInfo(ctx)
	if ai == nil {
		return nil
	}
	return authTimeFromToken(ai.Token)
}

// appx strips only an exact "Bearer " prefix, while the validator accepts the
// scheme in any case and with any spacing, so the scheme can still be here.
func authTimeFromToken(header string) *time.Time {
	var token string
	switch f := strings.Fields(header); {
	case len(f) == 1:
		token = f[0]
	case len(f) == 2 && strings.EqualFold(f[0], "bearer"):
		token = f[1]
	default:
		return nil
	}
	var claims authTimeClaims
	if _, _, err := jwt.NewParser().ParseUnverified(token, &claims); err != nil || claims.AuthTime == nil {
		return nil
	}
	t := claims.AuthTime.Time
	return &t
}
