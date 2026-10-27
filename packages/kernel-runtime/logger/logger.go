package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// Logger is a structured JSON logger. It satisfies contracts.Logger.
type Logger struct {
	sl *slog.Logger
}

// New creates a logger writing JSON lines to stderr at the given level
func New(level string) *Logger {
	return NewWithWriter(os.Stderr, level)
}

// NewWithWriter creates a logger writing JSON lines to w
func NewWithWriter(w io.Writer, level string) *Logger {
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: parseLevel(level)})
	return &Logger{sl: slog.New(h)}
}

// With returns a logger that adds the given key/value pairs to every record
func (l *Logger) With(keysAndValues ...interface{}) *Logger {
	return &Logger{sl: l.sl.With(keysAndValues...)}
}

// Debug logs at debug level
func (l *Logger) Debug(msg string, keysAndValues ...interface{}) { l.sl.Debug(msg, keysAndValues...) }

// Info logs at info level
func (l *Logger) Info(msg string, keysAndValues ...interface{}) { l.sl.Info(msg, keysAndValues...) }

// Warn logs at warn level
func (l *Logger) Warn(msg string, keysAndValues ...interface{}) { l.sl.Warn(msg, keysAndValues...) }

// Error logs at error level
func (l *Logger) Error(msg string, keysAndValues ...interface{}) { l.sl.Error(msg, keysAndValues...) }

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
