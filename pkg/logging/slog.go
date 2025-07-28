package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Config represents slog-compatible logging configuration
type Config struct {
	Level    string       `yaml:"level"`  // debug, info, warn, error
	Format   string       `yaml:"format"` // json, text
	Output   string       `yaml:"output"` // stdout, stderr, file, journald
	File     *LogFile     `yaml:"file,omitempty"`
	Journald *LogJournald `yaml:"journald,omitempty"`
}

// LogFile represents file logging configuration
type LogFile struct {
	Directory string `yaml:"directory"`
	Filename  string `yaml:"filename"`
	MaxSize   string `yaml:"max_size"`
	MaxFiles  int    `yaml:"max_files"`
	MaxAge    string `yaml:"max_age"`
	Compress  bool   `yaml:"compress"`
}

// LogJournald represents journald logging configuration
type LogJournald struct {
	Identifier string            `yaml:"identifier"`
	Fields     map[string]string `yaml:"fields"`
}

// NewLogger creates a configured slog.Logger with service name
func NewLogger(serviceName string, config Config) *slog.Logger {
	// Parse log level
	level := parseLogLevel(config.Level)

	// Create handler options
	opts := &slog.HandlerOptions{
		Level: level,
	}

	// Create writer based on output configuration
	writer := createWriter(config)

	// Create handler based on format
	var handler slog.Handler
	if strings.ToLower(config.Format) == "json" {
		handler = slog.NewJSONHandler(writer, opts)
	} else {
		handler = slog.NewTextHandler(writer, opts)
	}

	// Create logger with service context
	logger := slog.New(handler)
	return logger.With("service", serviceName)
}

// NewLoggerWithContext creates logger with default context fields
func NewLoggerWithContext(serviceName string, config Config, fields map[string]any) *slog.Logger {
	logger := NewLogger(serviceName, config)

	// Add context fields
	if len(fields) > 0 {
		var args []any
		for key, value := range fields {
			args = append(args, key, value)
		}
		return logger.With(args...)
	}

	return logger
}

// ContextLogger returns logger with context values
func ContextLogger(ctx context.Context, logger *slog.Logger) *slog.Logger {
	// Extract common context values and add them to the logger
	if userID := ctx.Value("user_id"); userID != nil {
		logger = logger.With("user_id", userID)
	}
	if sessionID := ctx.Value("session_id"); sessionID != nil {
		logger = logger.With("session_id", sessionID)
	}
	if requestID := ctx.Value("request_id"); requestID != nil {
		logger = logger.With("request_id", requestID)
	}
	if gameID := ctx.Value("game_id"); gameID != nil {
		logger = logger.With("game_id", gameID)
	}

	return logger
}

// NewServiceLogger creates a logger with standard service fields
func NewServiceLogger(serviceName, componentName string, config Config) *slog.Logger {
	return NewLoggerWithContext(serviceName, config, map[string]any{
		"component": componentName,
	})
}

// parseLogLevel converts string to slog.Level
func parseLogLevel(level string) slog.Level {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARNING", "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// createWriter creates the appropriate writer based on configuration
func createWriter(config Config) io.Writer {
	switch strings.ToLower(config.Output) {
	case "stdout":
		return os.Stdout
	case "stderr":
		return os.Stderr
	case "file":
		if config.File == nil {
			fmt.Fprintf(os.Stderr, "Warning: File configuration missing, falling back to stdout\n")
			return os.Stdout
		}
		writer, err := createFileWriter(config.File)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to create file writer (%v), falling back to stdout\n", err)
			return os.Stdout
		}
		return writer
	case "journald":
		if config.Journald == nil {
			fmt.Fprintf(os.Stderr, "Warning: Journald configuration missing, falling back to stdout\n")
			return os.Stdout
		}
		return createJournaldWriter(config.Journald)
	default:
		fmt.Fprintf(os.Stderr, "Warning: Unknown output type '%s', falling back to stdout\n", config.Output)
		return os.Stdout
	}
}

// createFileWriter creates a rotating file writer
func createFileWriter(config *LogFile) (io.Writer, error) {
	// Ensure directory exists
	if err := os.MkdirAll(config.Directory, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Parse max size
	maxSize, err := parseSize(config.MaxSize)
	if err != nil {
		return nil, fmt.Errorf("invalid max_size: %w", err)
	}

	// Parse max age
	maxAge, err := parseAge(config.MaxAge)
	if err != nil {
		return nil, fmt.Errorf("invalid max_age: %w", err)
	}

	filename := filepath.Join(config.Directory, config.Filename)

	return &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    maxSize,
		MaxBackups: config.MaxFiles,
		MaxAge:     maxAge,
		Compress:   config.Compress,
	}, nil
}

// createJournaldWriter creates a writer for journald
func createJournaldWriter(config *LogJournald) io.Writer {
	// For now, return stdout with identifier prefix since full journald integration
	// requires systemd dependencies that may not be available in all environments
	fmt.Fprintf(os.Stderr, "Info: Journald logging requested (identifier: %s), using stdout for compatibility\n", config.Identifier)
	return os.Stdout
}

// parseSize converts size string to megabytes
func parseSize(sizeStr string) (int, error) {
	sizeStr = strings.ToUpper(strings.TrimSpace(sizeStr))

	if strings.HasSuffix(sizeStr, "MB") {
		sizeStr = strings.TrimSuffix(sizeStr, "MB")
		var size int
		_, err := fmt.Sscanf(sizeStr, "%d", &size)
		return size, err
	}

	if strings.HasSuffix(sizeStr, "GB") {
		sizeStr = strings.TrimSuffix(sizeStr, "GB")
		var size int
		_, err := fmt.Sscanf(sizeStr, "%d", &size)
		return size * 1024, err
	}

	var size int
	_, err := fmt.Sscanf(sizeStr, "%d", &size)
	return size, err
}

// parseAge converts age string to days
func parseAge(ageStr string) (int, error) {
	ageStr = strings.ToLower(strings.TrimSpace(ageStr))

	if strings.HasSuffix(ageStr, "d") {
		ageStr = strings.TrimSuffix(ageStr, "d")
		var age int
		_, err := fmt.Sscanf(ageStr, "%d", &age)
		return age, err
	}

	if strings.HasSuffix(ageStr, "days") {
		ageStr = strings.TrimSuffix(ageStr, "days")
		var age int
		_, err := fmt.Sscanf(ageStr, "%d", &age)
		return age, err
	}

	var age int
	_, err := fmt.Sscanf(ageStr, "%d", &age)
	return age, err
}

// GetEnvOrDefault gets environment variable or returns default
func GetEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// LegacyConfig creates a Config from environment variables for backward compatibility
func LegacyConfig() Config {
	return Config{
		Level:  GetEnvOrDefault("LOG_LEVEL", "info"),
		Format: GetEnvOrDefault("LOG_FORMAT", "text"),
		Output: GetEnvOrDefault("LOG_OUTPUT", "stdout"),
	}
}

// NewLoggerBasic creates a logger with basic string parameters
func NewLoggerBasic(serviceName, level, format, output string) *slog.Logger {
	config := Config{
		Level:  level,
		Format: format,
		Output: output,
	}
	return NewLogger(serviceName, config)
}
