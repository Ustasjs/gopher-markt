package settings

import (
	"errors"
	"flag"
	"os"
	"strings"
)

var errorMessageServerAddress = "invalid server address. Expected format: host:port"

func validateServerAddress(serverAddress string) error {
	value := strings.Split(serverAddress, ":")
	if len(value) != 2 {
		return errors.New(errorMessageServerAddress)
	}
	host := value[0]
	port := value[1]
	if host == "" || port == "" {
		return errors.New(errorMessageServerAddress)
	}
	return nil
}

func initServerAddress(settings *Settings) {
	var serverAddressValue ServerAddress = "localhost:8080"
	settings.ServerAddress = serverAddressValue

	flag.Func("a", "Input server address", func(flagValue string) error {
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
