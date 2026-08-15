package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type jwtManager struct {
	secretKey []byte
	issuer    string
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
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(m.secretKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return signedToken, nil
}

func (m *jwtManager) ValidateToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(
		tokenStr,
		&Claims{},
		func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return m.secretKey, nil
		},
	)

	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidToken, err.Error())
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
