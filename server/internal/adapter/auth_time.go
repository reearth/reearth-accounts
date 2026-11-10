package adapter

import (
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type authTimeClaims struct {
	AuthTime *jwt.NumericDate `json:"auth_time"`
	jwt.RegisteredClaims
}

// AuthTime returns the auth_time claim of the request's token, or nil when
// there is no token or no readable claim. The JWT middleware has already
// verified the token, so the claims are read without verifying again.
func AuthTime(ctx context.Context) *time.Time {
	ai := GetAuthInfo(ctx)
	if ai == nil {
		return nil
	}
	return authTimeFromToken(ai.Token)
}

func authTimeFromToken(token string) *time.Time {
	if token == "" {
		return nil
	}
	var claims authTimeClaims
	if _, _, err := jwt.NewParser().ParseUnverified(token, &claims); err != nil || claims.AuthTime == nil {
		return nil
	}
	t := claims.AuthTime.Time
	return &t
}
