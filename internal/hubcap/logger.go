package hubcap

import (
	"log/slog"
	"os"
)

func SetupLogger(cfg *Config) *slog.Logger {
	logHandlerOptions := slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	switch cfg.LogLevel {
	case 0:
		logHandlerOptions.Level = slog.LevelWarn
	case 1:
		logHandlerOptions.Level = slog.LevelInfo
	case 2:
		logHandlerOptions.Level = slog.LevelDebug
	default:
		logHandlerOptions.AddSource = true
		logHandlerOptions.Level = slog.LevelDebug
	}
	var logger *slog.Logger
	logfile := os.Stdout
	var logfileError error
	if cfg.LogFile != "" {
		logfile, logfileError = os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if logfileError != nil {
			logfile = os.Stdout
		}
	}
	if cfg.LogJson {
		logger = slog.New(slog.NewJSONHandler(logfile, &logHandlerOptions))
	} else {
		logger = slog.New(slog.NewTextHandler(logfile, &logHandlerOptions))
	}
	if logfileError != nil {
		logger.Warn("Failed to open log file, defaulting to stdout", "logfile", cfg.LogFile, "error", logfileError)
	}
	return logger
}
