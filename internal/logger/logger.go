// Package logger provides persistent file-based logging for wizard operations.
// Logs are written to ~/.informa-wizard/logs/wizard.log in JSON format.
// The file is automatically rotated when it exceeds 5 MB; up to 3 rotated
// copies are kept (wizard.log.1, wizard.log.2, wizard.log.3).
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

const (
	maxLogSize      = 5 * 1024 * 1024 // 5 MB
	maxRotatedFiles = 3
	logDir          = ".informa-wizard/logs"
	logFileName     = "wizard.log"
)

var (
	mu     sync.Mutex
	global *fileLogger
)

type fileLogger struct {
	path   string
	file   *os.File
	logger *slog.Logger
}

// Init opens (or creates) the log file and sets up the global logger.
// If the log file or its directory cannot be created, Init returns an error
// but the package degrades gracefully: all log calls become no-ops.
func Init(homeDir string) error {
	dir := filepath.Join(homeDir, logDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}

	path := filepath.Join(dir, logFileName)

	// Rotate before opening if the existing file is already at the size limit.
	if info, err := os.Stat(path); err == nil && info.Size() >= maxLogSize {
		rotateFiles(path)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}

	handler := slog.NewJSONHandler(f, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	l := &fileLogger{
		path:   path,
		file:   f,
		logger: slog.New(handler),
	}

	mu.Lock()
	global = l
	mu.Unlock()

	return nil
}

// Close flushes and closes the log file.
func Close() error {
	mu.Lock()
	l := global
	global = nil
	mu.Unlock()

	if l == nil || l.file == nil {
		return nil
	}
	return l.file.Close()
}

// Info logs a message at INFO level.
func Info(format string, args ...any) {
	log(slog.LevelInfo, format, args...)
}

// Warn logs a message at WARN level.
func Warn(format string, args ...any) {
	log(slog.LevelWarn, format, args...)
}

// Error logs a message at ERROR level.
func Error(format string, args ...any) {
	log(slog.LevelError, format, args...)
}

func log(level slog.Level, format string, args ...any) {
	mu.Lock()
	l := global
	mu.Unlock()

	if l == nil {
		// Fallback: when the logger has not been initialized (e.g. early
		// failure resolving the home directory, or CLI flag-only paths that
		// skip Init), surface messages on stderr rather than dropping them.
		// Keep formatting minimal — a single line with a level prefix.
		var prefix string
		switch level {
		case slog.LevelInfo:
			prefix = "[INFO] "
		case slog.LevelWarn:
			prefix = "[WARN] "
		case slog.LevelError:
			prefix = "[ERROR] "
		default:
			prefix = "[LOG] "
		}
		fmt.Fprintf(os.Stderr, prefix+format+"\n", args...)
		return
	}

	msg := fmt.Sprintf(format, args...)

	// Rotate if the log file has grown past the limit.
	if l.file != nil {
		if info, err := l.file.Stat(); err == nil && info.Size() >= maxLogSize {
			rotateAndReopen(l)
		}
	}

	l.logger.Log(nil, level, msg) //nolint:sloglint
}

// rotateAndReopen renames the current log file and opens a fresh one.
// Must be called while holding enough context to act on l.
func rotateAndReopen(l *fileLogger) {
	_ = l.file.Close()
	l.file = nil
	rotateFiles(l.path)

	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		// Cannot reopen — silence all further log writes.
		l.logger = slog.New(slog.NewJSONHandler(io.Discard, nil))
		return
	}

	l.file = f
	l.logger = slog.New(slog.NewJSONHandler(f, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}

// rotateFiles renames wizard.log → wizard.log.1, wizard.log.1 → wizard.log.2, etc.
// Files beyond maxRotatedFiles are removed.
func rotateFiles(basePath string) {
	// Remove the oldest rotated file if it would exceed the limit.
	oldest := fmt.Sprintf("%s.%d", basePath, maxRotatedFiles)
	_ = os.Remove(oldest)

	// Shift existing rotated files up by one.
	for i := maxRotatedFiles - 1; i >= 1; i-- {
		src := fmt.Sprintf("%s.%d", basePath, i)
		dst := fmt.Sprintf("%s.%d", basePath, i+1)
		_ = os.Rename(src, dst)
	}

	// Rename the current log to .1
	_ = os.Rename(basePath, basePath+".1")
}
