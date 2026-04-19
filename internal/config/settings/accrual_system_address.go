package settings

import (
	"flag"
	"fmt"
	"net/url"
	"os"
)

var errorMessageAccrualSystemAddress = "invalid accrual system address. Expected format: host:port or http://host:port"

func validateAccrualSystemAddress(accrualSystemAddress string) error {
	// Try to parse as URL first (supports http://host:port)
	if parsedURL, err := url.Parse(accrualSystemAddress); err == nil && parsedURL.Host != "" {
		return nil
	}

	// Fallback to simple host:port validation
	err := validateBaseServerAddress(accrualSystemAddress, errorMessageAccrualSystemAddress)
	if err != nil {
		return fmt.Errorf("%s, got: %q", errorMessageAccrualSystemAddress, accrualSystemAddress)
	}
	return nil
}

func initAccrualSystemAddress(settings *Settings) {
	var accrualSystemAddressValue AccrualSystemAddress = "localhost:3000"
	settings.AccrualSystemAddress = accrualSystemAddressValue

	flag.Func("a", "Input accrual system address", func(flagValue string) error {
		err := validateAccrualSystemAddress(flagValue)
		if err != nil {
			return err
		}
		settings.AccrualSystemAddress = AccrualSystemAddress(flagValue)
		return nil
	})

	if envAccrualSystemAddress := os.Getenv("ACCRUAL_SYSTEM_ADDRESS"); envAccrualSystemAddress != "" {
		err := validateAccrualSystemAddress(envAccrualSystemAddress)
		if err != nil {
			panic(err)
		}
		settings.AccrualSystemAddress = AccrualSystemAddress(envAccrualSystemAddress)
	}
}
