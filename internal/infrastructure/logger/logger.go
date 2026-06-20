package logger

import (
	"log/slog"
	"os"
)

// New creates a JSON-structured logger at the requested level.
// Unrecognised level strings fall back to INFO.
func New(level string) *slog.Logger {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}
