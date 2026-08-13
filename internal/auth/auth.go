package auth

import (
	"errors"
	"time"
)

var (
	ErrInvalidToken = errors.New("invalid or expired token")
	ErrMissingToken = errors.New("authorization header or token is missing")
)

type Claims struct {
	UserID   string   `json:"user_id"`
	Username string   `json:"username"`
	Roles    []string `json:"roles,omitempty"`
}

type TokenManager interface {
	GenerateToken(userID string, username string, duration time.Duration) (string, error)
	ValidateToken(tokenStr string) (*Claims, error)
}
