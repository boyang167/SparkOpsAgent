package observability

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
)

func NewLogger(_ *config.Config) *zap.Logger {
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.TimeKey = "ts"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	logger, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	return logger
}

func NewNopLogger() *zap.Logger {
	return zap.NewNop()
}
