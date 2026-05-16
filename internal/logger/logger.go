package logger

import (
	"go.uber.org/zap"
)

var Log = zap.NewNop()

func Initialize(level zap.AtomicLevel) error {
	cfg := zap.NewProductionConfig()
	cfg.Level = level
	zl, err := cfg.Build()
	if err != nil {
		return err
	}

	Log = zl
	return nil
}
