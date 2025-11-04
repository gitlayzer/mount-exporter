package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/mount-exporter/mount-exporter/config"
	"github.com/mount-exporter/mount-exporter/recovery"
	"github.com/mount-exporter/mount-exporter/server"
)

var (
	version   = "dev"
	gitCommit = "unknown"
	buildTime = "unknown"
)

var (
	configFile = flag.String("config", "", "Path to configuration file")
	showHelp   = flag.Bool("help", false, "Show help message")
	showVersion = flag.Bool("version", false, "Show version information")
	logLevel   = flag.String("log-level", "", "Override log level (debug, info, warn, error, fatal)")
)

func main() {
	// Initialize panic recovery
	panicHandler := recovery.NewDefaultPanicHandler()

	// Set up global panic recovery
	defer panicHandler.Recover(&recovery.PanicInfo{
		Timestamp:  time.Now(),
		GoroutineID: "main",
		PanicValue:  nil,
		Message:     "Main goroutine panic",
	})

	// Use panic recovery for the main function
	err := panicHandler.RecoverWithFunc(func() error {
		return runApplication()
	})

	if err != nil {
		log.Fatalf("Application failed: %v", err)
	}
}

// runApplication contains the main application logic
func runApplication() error {
	flag.Parse()

	if *showHelp {
		showHelpMessage()
		return nil
	}

	if *showVersion {
		showVersionInfo()
		return nil
	}

	// Initialize logging
	logger := log.New(os.Stderr, "[mount-exporter] ", log.LstdFlags)

	// Initialize panic recovery with custom logger
	panicHandler := recovery.NewPanicHandler(recovery.PanicRecoveryConfig{
		Enabled: true,
		Logger:  &recoveryLogger{logger: logger},
		Handlers: []recovery.PanicHandlerFunc{
			// Custom handler for application-specific panic handling
			func(info recovery.PanicInfo) {
				logger.Printf("APPLICATION PANIC: %v at %s", info.PanicValue, info.Timestamp.Format(time.RFC3339))
			},
		},
	})

	// Load configuration
	cfg, err := loadConfiguration(*configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Override log level if specified
	if *logLevel != "" {
		cfg.Logging.Level = *logLevel
	}

	// Setup logging based on configuration
	if err := setupLogging(cfg.Logging, logger); err != nil {
		return fmt.Errorf("failed to setup logging: %w", err)
	}

	logger.Printf("Starting mount exporter version %s", version)
	logger.Printf("Git commit: %s", gitCommit)
	logger.Printf("Build time: %s", buildTime)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	logger.Printf("Configuration loaded successfully")
	logger.Printf("Server: %s:%d", cfg.Server.Host, cfg.Server.Port)
	logger.Printf("Mount points: %v", cfg.MountPoints)
	logger.Printf("Collection interval: %v", cfg.Interval)

	// Create and start server with panic recovery
	srv, err := server.NewServer(cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	// Set server version
	server.SetVersion(version)

	// Start the server with panic recovery
	if err := panicHandler.RecoverWithFunc(func() error {
		return srv.Start()
	}); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	// Wait for shutdown signal
	srv.WaitForShutdown()

	return nil
}

// loadConfiguration loads and validates the configuration
func loadConfiguration(configFile string) (*config.Config, error) {
	// Try to find config file if not specified
	if configFile == "" {
		configFile = findConfigFile()
	}

	cfg, err := config.LoadFromFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	return cfg, nil
}

// findConfigFile searches for configuration files in common locations
func findConfigFile() string {
	locations := []string{
		"config.yaml",
		"config.yml",
		"examples/config.yaml",
		"examples/config.yml",
		"/etc/mount-exporter/config.yaml",
		"/etc/mount-exporter/config.yml",
		"./config.yaml",
		"./config.yml",
	}

	for _, location := range locations {
		if _, err := os.Stat(location); err == nil {
			return location
		}
	}

	return ""
}

// setupLogging configures logging based on the configuration
func setupLogging(logConfig config.LoggingConfig, logger *log.Logger) error {
	// Set log format
	switch logConfig.Format {
	case "json":
		// For now, we'll keep the standard format
		// In a real implementation, you might use a structured logging library
		logger.SetFlags(log.LstdFlags)
	case "text":
		logger.SetFlags(log.LstdFlags)
	default:
		logger.SetFlags(log.LstdFlags)
	}

	// Set log level (basic implementation)
	// In a real implementation, you might use a proper logging library
	switch strings.ToLower(logConfig.Level) {
	case "debug":
		logger.SetFlags(log.LstdFlags | log.Lshortfile)
	case "info", "warn", "error", "fatal":
		logger.SetFlags(log.LstdFlags)
	default:
		logger.SetFlags(log.LstdFlags)
	}

	return nil
}

// showHelpMessage displays the help message
func showHelpMessage() {
	fmt.Printf(`Mount Exporter v%s

A Prometheus exporter for monitoring filesystem mount point availability.

USAGE:
    mount-exporter [OPTIONS]

OPTIONS:
    -config string
        Path to configuration file (default: searches for config.yaml in common locations)
    -log-level string
        Override log level (debug, info, warn, error, fatal)
    -help
        Show this help message
    -version
        Show version information

CONFIGURATION:
    The exporter looks for configuration files in these locations:
        - config.yaml
        - config.yml
        - examples/config.yaml
        - examples/config.yml
        - /etc/mount-exporter/config.yaml
        - /etc/mount-exporter/config.yml

EXAMPLE CONFIGURATION:
    server:
      host: "0.0.0.0"
      port: 8080
      path: "/metrics"
    mount_points:
      - "/data"
      - "/var/log"
      - "/mnt/backups"
    interval: 30s
    logging:
      level: "info"
      format: "json"

ENVIRONMENT VARIABLES:
    MOUNT_EXPORTER_HOST      Override server host
    MOUNT_EXPORTER_PORT      Override server port
    MOUNT_EXPORTER_PATH      Override metrics path
    MOUNT_EXPORTER_INTERVAL  Override collection interval
    MOUNT_EXPORTER_LOG_LEVEL Override log level

ENDPOINTS:
    /metrics    Prometheus metrics endpoint
    /health     Health check endpoint
    /           Basic information page

For more information, visit: https://github.com/mount-exporter/mount-exporter
`, version)
}

// showVersionInfo displays version information
func showVersionInfo() {
	fmt.Printf("mount-exporter %s\n", version)
	fmt.Printf("Git commit: %s\n", gitCommit)
	fmt.Printf("Build time: %s\n", buildTime)
}

// recoveryLogger adapts standard log.Logger to recovery.Logger interface
type recoveryLogger struct {
	logger *log.Logger
}

func (l *recoveryLogger) Printf(format string, args ...interface{}) {
	l.logger.Printf(format, args...)
}