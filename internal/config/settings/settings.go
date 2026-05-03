package settings

import (
	"flag"

	"go.uber.org/zap"
)

type ServerAddress string
type AccrualSystemAddress string
type DatabaseURI string

type Settings struct {
	ServerAddress        ServerAddress
	AccrualSystemAddress AccrualSystemAddress
	LogLevel             zap.AtomicLevel
	DatabaseURI          DatabaseURI
	JWTSecret            JWTSecret
}

func InitSettings() (*Settings, error) {
	settings := new(Settings)

	if err := initServerAddress(settings); err != nil {
		return nil, err
	}
	if err := initAccrualSystemAddress(settings); err != nil {
		return nil, err
	}
	if err := initLogLevel(settings); err != nil {
		return nil, err
	}
	initDatabaseURI(settings)
	initJWTSecret(settings)

	flag.Parse()

	return settings, nil
}
