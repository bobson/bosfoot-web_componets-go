package logger

import (
	"io"
	"log/slog"
	"os"
)

// Logger wraps the slog.Logger to maintain compatibility
type Logger struct {
	slog *slog.Logger
	file *os.File
}

// NewLogger creates a structured JSON logger that writes every record to both
// stdout (so it's visible via journalctl/docker logs) and logFilePath (for a
// persistent on-disk record).
func NewLogger(logFilePath string) (*Logger, error) {
	file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	w := io.MultiWriter(os.Stdout, file)
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo})

	return &Logger{
		slog: slog.New(handler),
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
