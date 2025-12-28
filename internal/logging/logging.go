// Package logging provides structured logging for wizado
// Supports multiple log levels, file rotation, and structured output
package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// Level represents log severity
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Entry represents a single log entry
type Entry struct {
	Timestamp time.Time         `json:"timestamp"`
	Level     string            `json:"level"`
	Message   string            `json:"message"`
	Component string            `json:"component,omitempty"`
	Fields    map[string]any    `json:"fields,omitempty"`
	Caller    string            `json:"caller,omitempty"`
}

// Logger provides structured logging
type Logger struct {
	mu        sync.Mutex
	level     Level
	output    io.Writer
	file      *os.File
	filePath  string
	component string
	fields    map[string]any
	maxSize   int64 // Max file size in bytes before rotation
	jsonMode  bool
}

// Config holds logger configuration
type Config struct {
	Level     Level
	FilePath  string
	MaxSizeMB int  // Max log file size in MB (default 10)
	JSONMode  bool // Output as JSON
	Component string
}

// DefaultConfig returns default logger configuration
func DefaultConfig() Config {
	home, _ := os.UserHomeDir()
	return Config{
		Level:     LevelInfo,
		FilePath:  filepath.Join(home, ".cache", "wizado", "wizado.log"),
		MaxSizeMB: 10,
		JSONMode:  false,
		Component: "wizado",
	}
}

var (
	defaultLogger *Logger
	once          sync.Once
)

// Init initializes the default logger
func Init(cfg Config) error {
	var err error
	once.Do(func() {
		defaultLogger, err = New(cfg)
	})
	return err
}

// Default returns the default logger, initializing if necessary
func Default() *Logger {
	if defaultLogger == nil {
		Init(DefaultConfig())
	}
	return defaultLogger
}

// New creates a new logger
func New(cfg Config) (*Logger, error) {
	if cfg.MaxSizeMB <= 0 {
		cfg.MaxSizeMB = 10
	}

	l := &Logger{
		level:     cfg.Level,
		filePath:  cfg.FilePath,
		component: cfg.Component,
		maxSize:   int64(cfg.MaxSizeMB) * 1024 * 1024,
		jsonMode:  cfg.JSONMode,
		fields:    make(map[string]any),
	}

	if cfg.FilePath != "" {
		if err := l.openFile(); err != nil {
			// Fall back to stderr
			l.output = os.Stderr
		}
	} else {
		l.output = os.Stderr
	}

	return l, nil
}

func (l *Logger) openFile() error {
	dir := filepath.Dir(l.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(l.filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	l.file = f
	l.output = f
	return nil
}

// Close closes the log file
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// WithComponent returns a new logger with the given component name
func (l *Logger) WithComponent(component string) *Logger {
	return &Logger{
		level:     l.level,
		output:    l.output,
		file:      l.file,
		filePath:  l.filePath,
		component: component,
		fields:    copyFields(l.fields),
		maxSize:   l.maxSize,
		jsonMode:  l.jsonMode,
	}
}

// WithField returns a new logger with the given field added
func (l *Logger) WithField(key string, value any) *Logger {
	fields := copyFields(l.fields)
	fields[key] = value
	return &Logger{
		level:     l.level,
		output:    l.output,
		file:      l.file,
		filePath:  l.filePath,
		component: l.component,
		fields:    fields,
		maxSize:   l.maxSize,
		jsonMode:  l.jsonMode,
	}
}

// WithFields returns a new logger with the given fields added
func (l *Logger) WithFields(fields map[string]any) *Logger {
	newFields := copyFields(l.fields)
	for k, v := range fields {
		newFields[k] = v
	}
	return &Logger{
		level:     l.level,
		output:    l.output,
		file:      l.file,
		filePath:  l.filePath,
		component: l.component,
		fields:    newFields,
		maxSize:   l.maxSize,
		jsonMode:  l.jsonMode,
	}
}

func copyFields(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// SetLevel sets the minimum log level
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// Debug logs a debug message
func (l *Logger) Debug(msg string) {
	l.log(LevelDebug, msg, nil)
}

// Debugf logs a formatted debug message
func (l *Logger) Debugf(format string, args ...any) {
	l.log(LevelDebug, fmt.Sprintf(format, args...), nil)
}

// Info logs an info message
func (l *Logger) Info(msg string) {
	l.log(LevelInfo, msg, nil)
}

// Infof logs a formatted info message
func (l *Logger) Infof(format string, args ...any) {
	l.log(LevelInfo, fmt.Sprintf(format, args...), nil)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string) {
	l.log(LevelWarn, msg, nil)
}

// Warnf logs a formatted warning message
func (l *Logger) Warnf(format string, args ...any) {
	l.log(LevelWarn, fmt.Sprintf(format, args...), nil)
}

// Error logs an error message
func (l *Logger) Error(msg string) {
	l.log(LevelError, msg, nil)
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(format string, args ...any) {
	l.log(LevelError, fmt.Sprintf(format, args...), nil)
}

func (l *Logger) log(level Level, msg string, extraFields map[string]any) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Check for rotation
	l.rotateIfNeeded()

	entry := Entry{
		Timestamp: time.Now().UTC(),
		Level:     level.String(),
		Message:   msg,
		Component: l.component,
	}

	// Merge fields
	if len(l.fields) > 0 || len(extraFields) > 0 {
		entry.Fields = make(map[string]any)
		for k, v := range l.fields {
			entry.Fields[k] = v
		}
		for k, v := range extraFields {
			entry.Fields[k] = v
		}
	}

	// Add caller info for debug and error levels
	if level == LevelDebug || level == LevelError {
		if _, file, line, ok := runtime.Caller(2); ok {
			entry.Caller = fmt.Sprintf("%s:%d", filepath.Base(file), line)
		}
	}

	// Format output
	var output string
	if l.jsonMode {
		data, _ := json.Marshal(entry)
		output = string(data)
	} else {
		output = l.formatPlain(entry)
	}

	fmt.Fprintln(l.output, output)
}

func (l *Logger) formatPlain(e Entry) string {
	ts := e.Timestamp.Format("2006-01-02 15:04:05")
	
	var result string
	if e.Component != "" {
		result = fmt.Sprintf("[%s] [%s] [%s] %s", ts, e.Level, e.Component, e.Message)
	} else {
		result = fmt.Sprintf("[%s] [%s] %s", ts, e.Level, e.Message)
	}

	if len(e.Fields) > 0 {
		for k, v := range e.Fields {
			result += fmt.Sprintf(" %s=%v", k, v)
		}
	}

	if e.Caller != "" {
		result += fmt.Sprintf(" (%s)", e.Caller)
	}

	return result
}

func (l *Logger) rotateIfNeeded() {
	if l.file == nil || l.maxSize <= 0 {
		return
	}

	stat, err := l.file.Stat()
	if err != nil {
		return
	}

	if stat.Size() < l.maxSize {
		return
	}

	// Close current file
	l.file.Close()

	// Rotate: rename current to .1, .1 to .2, etc.
	for i := 4; i >= 1; i-- {
		old := fmt.Sprintf("%s.%d", l.filePath, i)
		new := fmt.Sprintf("%s.%d", l.filePath, i+1)
		os.Rename(old, new)
	}
	os.Rename(l.filePath, l.filePath+".1")

	// Open new file
	l.openFile()
}

// LogPath returns the path to the log file
func (l *Logger) LogPath() string {
	return l.filePath
}

// Global helper functions that use the default logger

// Debug logs a debug message
func Debug(msg string) {
	Default().Debug(msg)
}

// Debugf logs a formatted debug message
func Debugf(format string, args ...any) {
	Default().Debugf(format, args...)
}

// Info logs an info message
func Info(msg string) {
	Default().Info(msg)
}

// Infof logs a formatted info message
func Infof(format string, args ...any) {
	Default().Infof(format, args...)
}

// Warn logs a warning message
func Warn(msg string) {
	Default().Warn(msg)
}

// Warnf logs a formatted warning message
func Warnf(format string, args ...any) {
	Default().Warnf(format, args...)
}

// Error logs an error message
func Error(msg string) {
	Default().Error(msg)
}

// Errorf logs a formatted error message
func Errorf(format string, args ...any) {
	Default().Errorf(format, args...)
}

// WithComponent returns a logger with the given component name
func WithComponent(component string) *Logger {
	return Default().WithComponent(component)
}

// WithField returns a logger with the given field added
func WithField(key string, value any) *Logger {
	return Default().WithField(key, value)
}

// WithFields returns a logger with the given fields added
func WithFields(fields map[string]any) *Logger {
	return Default().WithFields(fields)
}

// SessionLogger creates a logger for a gaming session
func SessionLogger(sessionID string) *Logger {
	return Default().WithComponent("session").WithField("session_id", sessionID)
}

