package logger

import (
	"log/slog"
	"os"
	"path/filepath"
)

var file *os.File

// Setup configures the default slog logger to write to a log file.
// For a TUI, we shouldn't log to stdout/stderr.
func Setup() error {
	// Use XDG cache dir or fallback
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}

	logDir := filepath.Join(cacheDir, "clickup-tui")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	logPath := filepath.Join(logDir, "app.log")
	file, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	// Create JSON logger
	handler := slog.NewJSONHandler(file, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	
	logger := slog.New(handler)
	slog.SetDefault(logger)

	return nil
}

// Close closes the log file.
func Close() {
	if file != nil {
		file.Close()
	}
}

// GetLogPath returns the path to the current log file
func GetLogPath() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	return filepath.Join(cacheDir, "clickup-tui", "app.log")
}
