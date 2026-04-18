package logger

import (
	"log/slog"
	"os"
	"path/filepath"
)

var (
	file              *os.File
	LogResponseBodies bool
	LogSensitiveData  bool
)

func init() {
	LogResponseBodies = os.Getenv("LOG_RESPONSE_BODIES") == "1"
	LogSensitiveData = os.Getenv("LOG_SENSITIVE_DATA") == "1"
}

// getLogPathInternal returns the path to the log file based on LOG_LOCAL environment variable.
func getLogPathInternal() string {
	if os.Getenv("LOG_LOCAL") == "1" {
		return "app.log"
	}

	// Use XDG cache dir or fallback
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}

	return filepath.Join(cacheDir, "clickup-tui", "app.log")
}

// Setup configures the default slog logger to write to a log file.
// For a TUI, we shouldn't log to stdout/stderr.
func Setup() error {
	logPath := getLogPathInternal()
	logDir := filepath.Dir(logPath)

	if logDir != "." {
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return err
		}
	}

	var err error
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
	return getLogPathInternal()
}

// TruncateBody returns the response body or "[REDACTED]" based on LOG_RESPONSE_BODIES.
func TruncateBody(body string) string {
	if LogResponseBodies {
		return body
	}
	return "[response body redacted, set LOG_RESPONSE_BODIES=1 to log]"
}

// RedactSensitive removes email addresses and long IDs if LOG_SENSITIVE_DATA is not set.
func RedactSensitive(data string) string {
	if LogSensitiveData {
		return data
	}
	// Simple redaction: replace email-like patterns and long IDs
	// For more complex needs, use a proper regex library
	return "[sensitive data redacted, set LOG_SENSITIVE_DATA=1 to log]"
}
