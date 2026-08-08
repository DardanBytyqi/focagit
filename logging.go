package main

import (
	"log/slog"
	"os"
	"strings"
)

// setupLogging installs the process-wide structured logger.
func setupLogging() *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(os.Getenv("FOCAGIT_LOG")) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	// stderr, not stdout: the terminal harness in cmd/focagit writes data to stdout.
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	}))
	slog.SetDefault(logger)
	return logger
}
