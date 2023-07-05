package main

import (
	"fmt"
	"os"

	"github.com/golang-jwt/jwt/v5"
)

var secret = os.Getenv("JWT_SECRET")

func auth(token string) (string, bool) {
	t, _ := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	claims, ok := t.Claims.(jwt.MapClaims)
	if !ok || !t.Valid {
		return "", false
	}
	md, ok := claims["user_metadata"].(map[string]any)
	if !ok {
		return "", false
	}
	auth, ok := md["beta"].(bool)
	if !ok {
		return "", false
	}
	sub, ok := md["sub"].(string)
	if !ok {
		return "", false
	}
	return sub, auth
}
