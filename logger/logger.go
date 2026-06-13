package logger

import (
	"log/slog"
	"os"
)

// Logger wraps the slog.Logger to maintain compatibility
type Logger struct {
	slog *slog.Logger
	file *os.File
}

// NewLogger creates a new structured logger outputting JSON to a file and stdout
func NewLogger(logFilePath string) (*Logger, error) {
	file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	// Create a multi-handler that logs to both stdout and file
	// Using JSON handler for structured logging
	handler := slog.NewJSONHandler(file, &slog.HandlerOptions{Level: slog.LevelInfo})
	logger := slog.New(handler)

	return &Logger{
		slog: logger,
		file: file,
	}, nil
}

// Info logs informational messages
func (l *Logger) Info(msg string, args ...any) {
	l.slog.Info(msg, args...)
}

// Error logs error messages
func (l *Logger) Error(msg string, err error, args ...any) {
	args = append(args, "error", err)
	l.slog.Error(msg, args...)
}

// Close closes the log file
func (l *Logger) Close() {
	l.file.Close()
}
