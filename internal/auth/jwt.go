package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTManager struct {
	secretKey []byte
	issuer    string
	parser    *jwt.Parser
}

func NewJWTManager(secretKey, issuer string) (*JWTManager, error) {
	if len(secretKey) < 32 {
		return nil, errors.New("secret key must be at least 32 bytes for HS256")
	}

	if issuer == "" {
		issuer = "janusgate"
	}

	return &JWTManager{
		secretKey: []byte(secretKey),
		issuer:    issuer,
		parser: jwt.NewParser(
			jwt.WithValidMethods([]string{"HS256"}),
			jwt.WithIssuer(issuer),
			jwt.WithExpirationRequired(),
			jwt.WithLeeway(5*time.Second),
		),
	}, nil
}

func (m *JWTManager) GenerateToken(ctx context.Context, userID, username string, roles []string, duration time.Duration) (string, error) {
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
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(m.secretKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return signedToken, nil
}

func (m *JWTManager) ValidateToken(ctx context.Context, tokenStr string) (Claims, error) {
	var claims Claims

	token, err := m.parser.ParseWithClaims(tokenStr, &claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.secretKey, nil
	})

	if err != nil {
		slog.DebugContext(ctx, "JWT validation failed", "error", err.Error())
		return Claims{}, ErrInvalidToken
	}

	if !token.Valid {
		slog.DebugContext(ctx, "JWT parsed but explicitly marked invalid")
		return Claims{}, ErrInvalidToken
	}

	return claims, nil
}
