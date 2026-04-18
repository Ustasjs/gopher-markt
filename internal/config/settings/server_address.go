package settings

import (
	"flag"
	"os"
)

var errorMessageServerAddress = "invalid server address. Expected format: host:port"

func validateServerAddress(serverAddress string) error {
	return validateBaseServerAddress(serverAddress, errorMessageServerAddress)
}

func initServerAddress(settings *Settings) {
	var serverAddressValue ServerAddress = "localhost:8080"
	settings.ServerAddress = serverAddressValue

	flag.Func("s", "Input server address", func(flagValue string) error {
		err := validateServerAddress(flagValue)
		if err != nil {
			return err
		}
		settings.ServerAddress = ServerAddress(flagValue)
		return nil
	})

	if envServerAddress := os.Getenv("RUN_ADDRESS"); envServerAddress != "" {
		err := validateServerAddress(envServerAddress)
		if err != nil {
			panic(err)
		}
		settings.ServerAddress = ServerAddress(envServerAddress)
	}
}
