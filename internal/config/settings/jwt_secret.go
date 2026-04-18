package settings

import (
	"os"
)

type JWTSecret string

func initJWTSecret(settings *Settings) {
	if envJWTSecret := os.Getenv("JWT_SECRET"); envJWTSecret != "" {
		settings.JWTSecret = JWTSecret(envJWTSecret)
	}
}
