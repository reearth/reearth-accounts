package adapter

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// LoginTimeSource reads the login time from the request's token. The zero
// value reads only auth_time. NewLoginTimeSource configures a claim to read in
// place of auth_time, for identity providers that cannot set auth_time
// themselves (Auth0 restricts it).
type LoginTimeSource struct {
	claim string
}

// Identity providers set these claims on every token, including refreshed
// ones, or they identify the token or session rather than the login, so none
// of them can show that the user logged in recently.
var reservedLoginTimeClaims = map[string]bool{
	"at_hash": true, "aud": true, "azp": true, "exp": true, "iat": true,
	"iss": true, "jti": true, "nbf": true, "nonce": true, "sid": true,
	"sub": true, "updated_at": true,
}

// NewLoginTimeSource returns a source that reads claim in place of auth_time,
// after trimming spaces. An empty claim, or auth_time itself, reads only
// auth_time. A reserved claim is an error.
func NewLoginTimeSource(claim string) (LoginTimeSource, error) {
	claim = strings.TrimSpace(claim)
	if reservedLoginTimeClaims[claim] {
		return LoginTimeSource{}, fmt.Errorf("login time claim must not be the reserved claim %q, which does not record when the user logged in", claim)
	}
	return LoginTimeSource{claim: claim}, nil
}

// Read returns the login time, or nil when there is no token or no usable
// claim. When a configured claim is present in the token, its value decides
// alone: a value that is not a NumericDate returns nil rather than falling
// back to auth_time. The claims are read without verifying the signature. In
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
	if s.claim != "" {
		if _, ok := claims[s.claim]; ok {
			name = s.claim
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
