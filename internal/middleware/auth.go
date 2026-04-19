package middleware

import (
	"context"
	"net/http"
	"strings"
)

const AuthCookieName = "auth_token"

type contextKey string

const UserIDContextKey contextKey = "user_id"

type UserRepository interface {
	CreateUser(ctx context.Context, login, passwordHash string) (userID string, err error)
}

type TokenParser interface {
	ParseUserID(tokenString string) (userID string, err error)
}

func GetUserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(UserIDContextKey).(string)
	return userID, ok
}

func Auth(parser TokenParser) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenString := strings.TrimPrefix(authHeader, "Bearer ")
				if userID, err := parser.ParseUserID(tokenString); err == nil && userID != "" {
					ctx := context.WithValue(r.Context(), UserIDContextKey, userID)
					r = r.WithContext(ctx)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireAuth() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, ok := GetUserIDFromContext(r.Context())
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
