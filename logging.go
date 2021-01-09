package stremio

import (
	"errors"
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewLogger creates a new logger with sane defaults and the passed level.
// Supported levels are: debug, info, warn, error.
// Only logs with that level and above are then logged (e.g. with "info" no debug logs will be logged).
// The encoding parameter is optional and will only be used when non-zero. Valid values: "console" (default) and "json".
//
// It makes sense to get this logger as early as possible and use it in your
// ManifestCallback, CatalogHandler and StreamHandler,
// so that all logs behave and are formatted the same way.
// You should then also set this logger in the options for `NewAddon()`,
// so that not two loggers are created.
// Alternatively you can create your own custom *zap.Logger and set it in the options
// when creating a new addon, leading to the addon using that custom logger.
func NewLogger(level, encoding string) (*zap.Logger, error) {
	logLevel, err := parseZapLevel(level)
	if err != nil {
		return nil, fmt.Errorf("Couldn't parse log level: %w", err)
	}
	logConfig := zap.NewDevelopmentConfig()
	logConfig.Level = zap.NewAtomicLevelAt(logLevel)
	// Deactivate stacktraces for warn level.
	logConfig.Development = false
	// Mix between zap's development and production EncoderConfig and other changes.
	logConfig.EncoderConfig = zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.RFC3339TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   nil,
	}
	if encoding != "" {
		logConfig.Encoding = encoding
	}
	// "console" encoding works without caller encoder, but "json" doesn't.
	// For "console" we prefer to have a more succinct log line without the caller (as configured above),
	// but for "json" (and potentially others in the future) we need to set it.
	if logConfig.Encoding != "console" {
		logConfig.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	}
	logger, err := logConfig.Build()
	if err != nil {
		return nil, fmt.Errorf("Couldn't create logger: %w", err)
	}

	return logger, nil
}

func parseZapLevel(logLevel string) (zapcore.Level, error) {
	switch logLevel {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info":
		return zapcore.InfoLevel, nil
	case "warn":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	}
	return 0, errors.New(`unknown log level - only knows ["debug", "info", "warn", "error"]`)
}
