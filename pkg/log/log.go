package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/op/go-logging"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Config represents the logging configuration from YAML
type Config struct {
	Level    string           `yaml:"level"`
	Format   string           `yaml:"format"`
	Output   string           `yaml:"output"`
	File     *FileConfig      `yaml:"file,omitempty"`
	Journald *JournaldConfig  `yaml:"journald,omitempty"`
}

// FileConfig represents file logging configuration
type FileConfig struct {
	Directory string `yaml:"directory"`
	Filename  string `yaml:"filename"`
	MaxSize   string `yaml:"max_size"`
	MaxFiles  int    `yaml:"max_files"`
	MaxAge    string `yaml:"max_age"`
	Compress  bool   `yaml:"compress"`
}

// JournaldConfig represents journald logging configuration
type JournaldConfig struct {
	Identifier string            `yaml:"identifier"`
	Fields     map[string]string `yaml:"fields"`
}

// SetupLogger creates a configured logger using go-logging
func SetupLogger(serviceName string, config Config) *logging.Logger {
	logger := logging.MustGetLogger(serviceName)
	
	// Choose format based on config
	var format logging.Formatter
	if strings.ToLower(config.Format) == "json" {
		// JSON-like format for structured logging
		format = logging.MustStringFormatter(
			`{"time":"%{time:2006-01-02T15:04:05.000Z07:00}","level":"%{level}","service":"%{module}","function":"%{shortfunc}","message":"%{message}"}`,
		)
	} else {
		// Text format with colors for development
		format = logging.MustStringFormatter(
			`%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.9s} %{id:07x}%{color:reset} %{message}`,
		)
	}

	// Create writer based on output configuration
	var writer io.Writer
	var err error

	switch strings.ToLower(config.Output) {
	case "stdout":
		writer = os.Stdout
	case "stderr":
		writer = os.Stderr
	case "file":
		if config.File == nil {
			fmt.Fprintf(os.Stderr, "Warning: File configuration missing, falling back to stdout\n")
			writer = os.Stdout
		} else {
			writer, err = createFileWriter(config.File)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to create file writer (%v), falling back to stdout\n", err)
				writer = os.Stdout
			}
		}
	case "journald":
		if config.Journald == nil {
			fmt.Fprintf(os.Stderr, "Warning: Journald configuration missing, falling back to stdout\n")
			writer = os.Stdout
		} else {
			writer = createJournaldWriter(config.Journald)
		}
	default:
		fmt.Fprintf(os.Stderr, "Warning: Unknown output type '%s', falling back to stdout\n", config.Output)
		writer = os.Stdout
	}

	// Create backend with writer and format
	backend := logging.NewLogBackend(writer, "", 0)
	backendFormatter := logging.NewBackendFormatter(backend, format)
	backendLeveled := logging.AddModuleLevel(backendFormatter)

	// Set log level
	level := parseLogLevel(config.Level)
	backendLeveled.SetLevel(level, "")

	// Attach backend to logger
	logger.SetBackend(backendLeveled)
	
	return logger
}

// SetupLoggerLegacy creates a logger using environment variables (for backward compatibility)
func SetupLoggerLegacy() *logging.Logger {
	config := Config{
		Level:  getEnvOrDefault("LOG_LEVEL", "INFO"),
		Format: getEnvOrDefault("LOG_FORMAT", "text"),
		Output: getEnvOrDefault("LOG_OUTPUT", "stdout"),
	}
	
	return SetupLogger("Controller", config)
}

// parseLogLevel converts string to logging level
func parseLogLevel(level string) logging.Level {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return logging.DEBUG
	case "INFO":
		return logging.INFO
	case "WARNING", "WARN":
		return logging.WARNING
	case "ERROR":
		return logging.ERROR
	case "CRITICAL":
		return logging.CRITICAL
	default:
		return logging.INFO
	}
}

// createFileWriter creates a rotating file writer
func createFileWriter(config *FileConfig) (io.Writer, error) {
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
func createJournaldWriter(config *JournaldConfig) io.Writer {
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

// getEnvOrDefault gets environment variable or returns default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
