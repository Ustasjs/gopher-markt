package settings

import (
	"flag"
	"os"
)

var errorMessageAccrualSystemAddress = "invalid accrual system address. Expected format: host:port"

func validateAccrualSystemAddress(accrualSystemAddress string) error {
	return validateBaseServerAddress(accrualSystemAddress, errorMessageAccrualSystemAddress)
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
