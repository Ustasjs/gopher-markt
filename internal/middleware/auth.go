package middleware

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const userIDContextKey contextKey = "user_id"

type TokenParser interface {
	ParseUserID(tokenString string) (userID string, err error)
}

func GetUserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(userIDContextKey).(string)
	return userID, ok
}

func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDContextKey, userID)
}

func Auth(parser TokenParser) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			tokenString, hasPrefix := strings.CutPrefix(authHeader, "Bearer ")
			if hasPrefix {
				if userID, err := parser.ParseUserID(tokenString); err == nil && userID != "" {
					ctx := ContextWithUserID(r.Context(), userID)
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
