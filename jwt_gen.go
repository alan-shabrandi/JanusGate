package main

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func main() {
	secretKey := []byte("super-secret-key-janusgate-32bytes!!")
	now := time.Now().UTC()

	claims := jwt.MapClaims{
		"user_id":  "user-123",
		"username": "alan",
		"roles":    []string{"admin"},
		"exp":      now.Add(1 * time.Hour).Unix(),
		"iat":      now.Unix(),
		"nbf":      now.Unix(),
		"iss":      "janusgate",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(secretKey)
	if err != nil {
		panic(err)
	}

	fmt.Println("New Valid Bearer Token:")
	fmt.Println(tokenString)
}
