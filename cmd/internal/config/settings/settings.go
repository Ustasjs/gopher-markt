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
}

func InitSettings() *Settings {
	settings := new(Settings)

	initServerAddress(settings)
	initAccrualSystemAddress(settings)
	initLogLevel(settings)
	initDatabaseURI(settings)

	flag.Parse()

	return settings
}
