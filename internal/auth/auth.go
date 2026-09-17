package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid or expired token")
	ErrMissingToken = errors.New("authorization header or token is missing")
)

type Claims struct {
	UserID   string   `json:"user_id"`
	Username string   `json:"username"`
	Roles    []string `json:"roles,omitempty"`
	jwt.RegisteredClaims
}

type TokenValidator interface {
	ValidateToken(tokenStr string) (Claims, error)
}

type TokenManager interface {
	TokenValidator
	GenerateToken(userID string, username string, roles []string, duration time.Duration) (string, error)
}
