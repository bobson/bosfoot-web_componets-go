package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// tgDedupWindow suppresses repeat Telegram alerts for the same message, so an
// error storm (e.g. DB pool exhaustion firing the same error in a loop) can't
// spam the bot or trip Telegram's rate limits.
const tgDedupWindow = 60 * time.Second

// Logger wraps the slog.Logger to maintain compatibility
type Logger struct {
	slog   *slog.Logger
	file   *os.File
	tgTok  string
	tgID   string
	appEnv string

	tgMu   sync.Mutex
	tgSeen map[string]time.Time // alert message -> last time it was sent
}

// NewLogger creates a structured JSON logger that writes every record to both
// stdout (so it's visible via journalctl/docker logs) and logFilePath (for a
// persistent on-disk record). It also picks up Telegram credentials if present.
func NewLogger(logFilePath string) (*Logger, error) {
	file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "production"
	}

	w := io.MultiWriter(os.Stdout, file)
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo})

	return &Logger{
		slog:   slog.New(handler),
		file:   file,
		tgTok:  os.Getenv("TELEGRAM_BOT_TOKEN"),
		tgID:   os.Getenv("TELEGRAM_CHAT_ID"),
		appEnv: env,
		tgSeen: make(map[string]time.Time),
	}, nil
}

// Info logs informational messages
func (l *Logger) Info(msg string, args ...any) {
	l.slog.Info(msg, args...)
}

// Error logs error messages and sends a Telegram alert if configured.
func (l *Logger) Error(msg string, err error, args ...any) {
	args = append(args, "error", err)
	l.slog.Error(msg, args...)

	if l.tgTok != "" && l.tgID != "" && l.shouldAlert(msg) {
		go l.sendTelegramAlert(msg, err)
	}
}

// shouldAlert reports whether a Telegram alert for msg should be sent now, and
// records the send. It de-duplicates identical messages within tgDedupWindow and
// prunes stale entries so the map can't grow unbounded.
func (l *Logger) shouldAlert(msg string) bool {
	now := time.Now()
	l.tgMu.Lock()
	defer l.tgMu.Unlock()

	if last, ok := l.tgSeen[msg]; ok && now.Sub(last) < tgDedupWindow {
		return false
	}
	for k, t := range l.tgSeen {
		if now.Sub(t) >= tgDedupWindow {
			delete(l.tgSeen, k)
		}
	}
	l.tgSeen[msg] = now
	return true
}

func (l *Logger) sendTelegramAlert(msg string, err error) {
	// Best-effort: 10s timeout, don't let a Telegram outage hang the app
	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", l.tgTok)

	// Plain text only — no parse_mode. Go error strings routinely contain
	// Markdown metacharacters (_ * ` [), which would make Telegram reject the
	// message with a 400 exactly when an error is happening.
	text := fmt.Sprintf("🚨 Bosfoot Alert [%s]\n\nMessage: %s\nError: %v\n\nTime: %s",
		strings.ToUpper(l.appEnv), msg, err, time.Now().Format(time.RFC822))

	payload := map[string]string{
		"chat_id": l.tgID,
		"text":    text,
	}

	b, _ := json.Marshal(payload)
	resp, postErr := client.Post(url, "application/json", bytes.NewBuffer(b))
	if postErr != nil {
		l.slog.Warn("failed to send telegram alert", "error", postErr)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		l.slog.Warn("telegram alert returned non-200", "status", resp.Status)
	}
}

// Close closes the log file
func (l *Logger) Close() {
	l.file.Close()
}
