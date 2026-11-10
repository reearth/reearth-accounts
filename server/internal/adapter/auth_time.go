package adapter

import (
	"context"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// AuthTime returns the login time from the request's token, or nil when there
// is no token or no readable claim. It reads the claim named by claim when that
// is set and holds a number, and the standard auth_time claim otherwise. The claims are read without
// verifying the signature. In production the JWT middleware has verified the
// token before this runs. In debug mode the X-Reearth-Debug-Auth-Token header
// can supply any token, but debug headers already allow impersonating any
// user, so debug mode is not a security boundary.
func AuthTime(ctx context.Context, claim string) *time.Time {
	ai := GetAuthInfo(ctx)
	if ai == nil {
		return nil
	}
	return authTimeFromToken(ai.Token, claim)
}

// appx strips only an exact "Bearer " prefix, while the validator accepts the
// scheme in any case and with any spacing, so the scheme can still be here.
func authTimeFromToken(header, claim string) *time.Time {
	var token string
	switch f := strings.Fields(header); {
	case len(f) == 1:
		token = f[0]
	case len(f) == 2 && strings.EqualFold(f[0], "bearer"):
		token = f[1]
	default:
		return nil
	}
	claims := jwt.MapClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(token, claims); err != nil {
		return nil
	}
	for _, name := range []string{claim, "auth_time"} {
		if sec, ok := claims[name].(float64); ok && name != "" {
			t := time.Unix(int64(sec), 0)
			return &t
		}
	}
	return nil
}
