package settings

import (
	"errors"
	"strings"
)

func validateBaseServerAddress(serverAddress string, errorMessage string) error {
	value := strings.Split(serverAddress, ":")
	if len(value) != 2 {
		return errors.New(errorMessage)
	}
	host := value[0]
	port := value[1]
	if host == "" || port == "" {
		return errors.New(errorMessage)
	}
	return nil
}
