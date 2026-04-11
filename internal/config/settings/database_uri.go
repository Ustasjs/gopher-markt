package settings

import (
	"flag"
	"os"
)

func initDatabaseURI(settings *Settings) {
	flag.Func("d", "Input database uri", func(flagValue string) error {
		settings.DatabaseURI = DatabaseURI(flagValue)
		return nil
	})

	if envBaseDatabaseDsn := os.Getenv("DATABASE_URI"); envBaseDatabaseDsn != "" {
		settings.DatabaseURI = DatabaseURI(envBaseDatabaseDsn)
	}
}
