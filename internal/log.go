package internal

import (
	"log/slog"
	"os"
	"strings"
)

var (
	logger *slog.Logger
)

func init() {
	levelVal := os.Getenv("LOG_LEVEL")
	level := parseLogLevel(levelVal)

	logger = slog.New(
		slog.NewJSONHandler(os.Stderr,
			&slog.HandlerOptions{
				AddSource:   true,
				Level:       level,
				ReplaceAttr: nil,
			}))
}

func Logger() *slog.Logger {
	return logger
}

func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
