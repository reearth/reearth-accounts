package adapter

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// LoginTimeSource reads the login time from the request's token. Claim names
// a claim to read in place of auth_time, for identity providers that cannot
// set auth_time themselves (Auth0 restricts it). Empty reads auth_time.
type LoginTimeSource struct {
	Claim string
}

// Read returns the login time, or nil when there is no token or no usable
// claim. When Claim is set and present in the token, its value decides alone:
// a value that is not a NumericDate returns nil rather than falling back to
// auth_time. The claims are read without verifying the signature. In
// production the JWT middleware has verified the token before this runs. In
// debug mode the X-Reearth-Debug-Auth-Token header can supply any token, but
// debug headers already allow impersonating any user, so debug mode is not a
// security boundary.
func (s LoginTimeSource) Read(ctx context.Context) *time.Time {
	ai := GetAuthInfo(ctx)
	if ai == nil {
		return nil
	}
	claims := claimsFromToken(ai.Token)
	if claims == nil {
		return nil
	}
	name := "auth_time"
	if s.Claim != "" {
		if _, ok := claims[s.Claim]; ok {
			name = s.Claim
		}
	}
	return numericDate(claims[name])
}

// appx strips only an exact "Bearer " prefix, while the validator accepts the
// scheme in any case and with any spacing, so the scheme can still be here.
func claimsFromToken(header string) jwt.MapClaims {
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
	return claims
}

func numericDate(v any) *time.Time {
	sec, ok := v.(float64)
	if !ok || math.IsNaN(sec) || sec >= math.MaxInt64 || sec < math.MinInt64 {
		return nil
	}
	t := time.Unix(int64(sec), 0)
	return &t
}
