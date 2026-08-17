package auth

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenValidator interface {
	ValidateToken(tokenStr string) (Claims, error)
}

type jwtManager struct {
	secretKey []byte
	issuer    string
	parser    *jwt.Parser
}

func NewJWTManager(secretKey, issuer string) (*jwtManager, error) {
	if len(secretKey) < 32 {
		return nil, errors.New("secret key must be at least 32 bytes for HS256")
	}

	if issuer == "" {
		issuer = "janusgate"
	}

	return &jwtManager{
		secretKey: []byte(secretKey),
		issuer:    issuer,
		parser: jwt.NewParser(
			jwt.WithValidMethods([]string{"HS256"}),
			jwt.WithIssuer(issuer),
			jwt.WithExpirationRequired(),
		),
	}, nil
}

func (m *jwtManager) GenerateToken(userID, username string, roles []string, duration time.Duration) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		UserID:   userID,
		Username: username,
		Roles:    roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    m.issuer,
			// ID: uuid.NewString(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(m.secretKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return signedToken, nil
}

func (m *jwtManager) ValidateToken(tokenStr string) (Claims, error) {
	var claims Claims

	token, err := m.parser.ParseWithClaims(tokenStr, &claims, func(token *jwt.Token) (interface{}, error) {
		return m.secretKey, nil
	})

	if err != nil {
		slog.Debug("JWT validation failed", "error", err.Error())
		return Claims{}, ErrInvalidToken
	}

	if !token.Valid {
		slog.Debug("JWT parsed but explicitly marked invalid")
		return Claims{}, ErrInvalidToken
	}

	return claims, nil
}
