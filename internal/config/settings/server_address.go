package settings

import (
	"flag"
	"os"
)

var errorMessageServerAddress = "invalid server address. Expected format: host:port"

func validateServerAddress(serverAddress string) error {
	return validateBaseServerAddress(serverAddress, errorMessageServerAddress)
}

func initServerAddress(settings *Settings) error {
	settings.ServerAddress = "localhost:8080"

	flag.Func("a", "Input server address", func(flagValue string) error {
		if err := validateServerAddress(flagValue); err != nil {
			return err
		}
		settings.ServerAddress = ServerAddress(flagValue)
		return nil
	})

	if envServerAddress := os.Getenv("RUN_ADDRESS"); envServerAddress != "" {
		if err := validateServerAddress(envServerAddress); err != nil {
			return err
		}
		settings.ServerAddress = ServerAddress(envServerAddress)
	}
	return nil
}
