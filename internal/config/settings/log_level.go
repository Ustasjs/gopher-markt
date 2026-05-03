package settings

import (
	"flag"
	"os"

	"go.uber.org/zap"
)

func initLogLevel(settings *Settings) error {
	settings.LogLevel = zap.NewAtomicLevel()

	flag.Func("l", "Input log level", func(flagValue string) error {
		lvl, err := zap.ParseAtomicLevel(flagValue)
		if err != nil {
			return err
		}
		settings.LogLevel = lvl
		return nil
	})

	if envLogLevel := os.Getenv("LOG_LEVEL"); envLogLevel != "" {
		lvl, err := zap.ParseAtomicLevel(envLogLevel)
		if err != nil {
			return err
		}
		settings.LogLevel = lvl
	}
	return nil
}
