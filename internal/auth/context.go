package auth

import (
	"context"
	"strings"
)

type contextKey struct{}

var userClaimsKey = contextKey{}

func InjectClaims(ctx context.Context, claims Claims) context.Context {
	return context.WithValue(ctx, userClaimsKey, claims)
}

func GetUserClaims(ctx context.Context) (Claims, bool) {
	claims, ok := ctx.Value(userClaimsKey).(Claims)
	return claims, ok
}

func GetUserID(ctx context.Context) (string, bool) {
	if claims, ok := GetUserClaims(ctx); ok {
		return claims.UserID, true
	}
	return "", false
}

func GetUsername(ctx context.Context) (string, bool) {
	if claims, ok := GetUserClaims(ctx); ok {
		return claims.Username, true
	}
	return "", false
}

func GetUserRoles(ctx context.Context) ([]string, bool) {
	if claims, ok := GetUserClaims(ctx); ok {
		safeRoles := make([]string, len(claims.Roles))
		copy(safeRoles, claims.Roles)
		return safeRoles, true
	}
	return nil, false
}

func HasRole(ctx context.Context, role string) bool {
	claims, ok := GetUserClaims(ctx)
	if !ok {
		return false
	}
	for _, r := range claims.Roles {
		if strings.EqualFold(r, role) {
			return true
		}
	}
	return false
}
