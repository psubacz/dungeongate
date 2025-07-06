package dungeongate

import (
	"os"
	"strings"

	"github.com/op/go-logging" // Log Levels
)

func SetupLogger() *logging.Logger {
	// Create a new logger
	var log = logging.MustGetLogger("Controller")
	// Set up a custom log format
	format := logging.MustStringFormatter(
		`%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.9s} %{id:07x}%{color:reset} %{message}`,
	)

	// Create a backend for logging to stdout with the custom format
	backend := logging.NewLogBackend(os.Stdout, "", 0)

	backendFormatter := logging.NewBackendFormatter(backend, format)

	// Set up a module level for the backend
	backends := logging.AddModuleLevel(backendFormatter)

	// Get log level from the environment variable
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "INFO" // Default to INFO if not set
		log.Warning("Log level not set, using default value of", logLevel)
	}

	// Set the log level based on the environment variable
	switch strings.ToUpper(logLevel) {
	case "DEBUG":
		backends.SetLevel(logging.DEBUG, "")
	case "INFO":
		backends.SetLevel(logging.INFO, "")
	case "WARNING":
		backends.SetLevel(logging.WARNING, "")
	case "ERROR":
		backends.SetLevel(logging.ERROR, "")
	case "CRITICAL":
		backends.SetLevel(logging.CRITICAL, "")
	default:
		log.Fatalf("Invalid log level: %s", logLevel)
	}

	// Attach the backend to the logger
	log.SetBackend(backends)
	log.Info("Log Level set to: ", logLevel)
	return log
}
