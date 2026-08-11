package observability

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/applog"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
)

var (
	globalLogStore *applog.Store
	logStoreMu     sync.RWMutex
)

func SetAppLogStore(store *applog.Store) {
	logStoreMu.Lock()
	globalLogStore = store
	logStoreMu.Unlock()
}

func AppLogStore() *applog.Store {
	logStoreMu.RLock()
	defer logStoreMu.RUnlock()
	return globalLogStore
}

func NewLogger(cfg *config.Config) *zap.Logger {
	logCfg := cfg.Observability.Log.WithDefaults()
	level := parseLevel(logCfg.Level)

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "ts"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderCfg.LevelKey = "level"
	encoderCfg.MessageKey = "msg"
	encoderCfg.CallerKey = "caller"

	jsonEncoder := zapcore.NewJSONEncoder(encoderCfg)
	cores := make([]zapcore.Core, 0, 3)

	if logCfg.Console {
		consoleEncoder := zapcore.NewConsoleEncoder(encoderCfg)
		cores = append(cores, zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), level))
	}

	if err := os.MkdirAll(logCfg.Directory, 0o755); err == nil {
		fileWriter := zapcore.AddSync(&lumberjack.Logger{
			Filename:   filepath.Join(logCfg.Directory, "sparkops.log"),
			MaxSize:    logCfg.MaxSizeMB,
			MaxBackups: logCfg.MaxBackups,
			Compress:   true,
		})
		cores = append(cores, zapcore.NewCore(jsonEncoder, fileWriter, level))
	}

	core := zapcore.NewTee(cores...)
	return zap.New(&dbHookCore{Core: core}, zap.AddCaller())
}

type dbHookCore struct {
	zapcore.Core
}

func (c *dbHookCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	err := c.Core.Write(ent, fields)
	store := AppLogStore()
	if store != nil && ent.Level >= zapcore.InfoLevel {
		entry := applog.Entry{
			Level:   ent.Level.String(),
			Message: ent.Message,
			Caller:  ent.Caller.TrimmedPath(),
			Fields:  fieldsToMap(fields),
		}
		if v, ok := entry.Fields["category"].(string); ok {
			entry.Category = v
		} else {
			entry.Category = applog.CategoryApp
		}
		if v, ok := entry.Fields["trace_id"].(string); ok {
			entry.TraceID = v
		}
		if v, ok := entry.Fields["workspace_id"].(string); ok {
			entry.WorkspaceID = v
		}
		go func() {
			_ = store.Append(context.Background(), entry)
		}()
	}
	return err
}

func fieldsToMap(fields []zapcore.Field) map[string]any {
	if len(fields) == 0 {
		return map[string]any{}
	}
	enc := zapcore.NewMapObjectEncoder()
	for _, f := range fields {
		f.AddTo(enc)
	}
	return enc.Fields
}

func parseLevel(raw string) zapcore.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return zapcore.DebugLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

func LogDirectory(cfg *config.Config) string {
	return cfg.Observability.Log.WithDefaults().Directory
}

func LoggerWithTrace(logger *zap.Logger, traceID, workspaceID, category string) *zap.Logger {
	fields := []zap.Field{zap.String("category", category)}
	if traceID != "" {
		fields = append(fields, zap.String("trace_id", traceID))
	}
	if workspaceID != "" {
		fields = append(fields, zap.String("workspace_id", workspaceID))
	}
	return logger.With(fields...)
}

func NewNopLogger() *zap.Logger {
	return zap.NewNop()
}
