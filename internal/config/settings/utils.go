package settings

import (
	"errors"
	"strings"

	"github.com/ustasjs/gopher-markt/internal/logger"
	"go.uber.org/zap"
)

func validateBaseServerAddress(serverAddress string, errorMessage string) error {
	value := strings.Split(serverAddress, ":")
	if len(value) != 2 {
		logger.Log.Warn("serverAddress format is wrong", zap.String("serverAddress", serverAddress))
		return errors.New(errorMessage)
	}
	host := value[0]
	port := value[1]
	if host == "" || port == "" {
		logger.Log.Warn("host aor port are empty", zap.String("serverAddress", serverAddress), zap.String("host", host), zap.String("port", port))
		return errors.New(errorMessage)
	}
	return nil
}
