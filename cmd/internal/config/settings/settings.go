package settings

import (
	"flag"

	"go.uber.org/zap"
)

type ServerAddress string
type DatabaseURI string

type Settings struct {
	ServerAddress ServerAddress
	LogLevel      zap.AtomicLevel
	DatabaseURI   DatabaseURI
}

func InitSettings() *Settings {
	settings := new(Settings)

	initServerAddress(settings)
	initLogLevel(settings)
	initDatabaseURI(settings)

	flag.Parse()

	return settings
}
