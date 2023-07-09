package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const (
	contextSubKey contextKey = "sub"
)

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		sub, auth := auth(token)
		if !auth {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), contextSubKey, sub)
		next(w, r.WithContext(ctx))
	}
}

func auth(token string) (string, bool) {
	t, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil || !t.Valid {
		return "", false
	}
	claims, ok := t.Claims.(jwt.MapClaims)
	if !ok || !t.Valid {
		return "", false
	}
	md, ok := claims["app_metadata"].(map[string]any)
	if !ok {
		return "", false
	}
	auth, ok := md["beta"].(bool)
	if !ok {
		return "", false
	}
	sub, ok := claims["sub"].(string)
	if !ok {
		return "", false
	}
	return sub, auth
}
