package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Server      ServerConfig      `yaml:"server"`
	MountPoints []string          `yaml:"mount_points"`
	Interval    time.Duration     `yaml:"interval"`
	Logging     LoggingConfig     `yaml:"logging"`
	mu          sync.RWMutex      `yaml:"-"`
}

// ServerConfig represents HTTP server configuration
type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
	Path string `yaml:"path"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
			Path: "/metrics",
		},
		MountPoints: []string{},
		Interval:    30 * time.Second,
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}
}

// LoadFromFile loads configuration from a YAML file
func LoadFromFile(filename string) (*Config, error) {
	config := DefaultConfig()

	if filename == "" {
		return config, nil
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return nil, fmt.Errorf("failed to read config file %s: %w", filename, err)
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", filename, err)
	}

	// Apply environment variable overrides
	if err := config.applyEnvOverrides(); err != nil {
		return nil, fmt.Errorf("failed to apply environment overrides: %w", err)
	}

	return config, nil
}

// applyEnvOverrides applies environment variable overrides
func (c *Config) applyEnvOverrides() error {
	if host := os.Getenv("MOUNT_EXPORTER_HOST"); host != "" {
		c.Server.Host = host
	}

	if port := os.Getenv("MOUNT_EXPORTER_PORT"); port != "" {
		p, err := strconv.Atoi(port)
		if err != nil {
			return fmt.Errorf("invalid port number in MOUNT_EXPORTER_PORT: %s", port)
		}
		c.Server.Port = p
	}

	if path := os.Getenv("MOUNT_EXPORTER_PATH"); path != "" {
		c.Server.Path = path
	}

	if interval := os.Getenv("MOUNT_EXPORTER_INTERVAL"); interval != "" {
		d, err := time.ParseDuration(interval)
		if err != nil {
			return fmt.Errorf("invalid interval in MOUNT_EXPORTER_INTERVAL: %s", interval)
		}
		c.Interval = d
	}

	if level := os.Getenv("MOUNT_EXPORTER_LOG_LEVEL"); level != "" {
		c.Logging.Level = level
	}

	return nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server port must be between 1 and 65535, got %d", c.Server.Port)
	}

	if c.Server.Path == "" {
		return fmt.Errorf("server path cannot be empty")
	}

	if c.Server.Path[0] != '/' {
		return fmt.Errorf("server path must start with '/', got %s", c.Server.Path)
	}

	if c.Interval <= 0 {
		return fmt.Errorf("interval must be positive, got %v", c.Interval)
	}

	if len(c.MountPoints) == 0 {
		return fmt.Errorf("at least one mount point must be configured")
	}

	for _, mp := range c.MountPoints {
		if mp == "" {
			return fmt.Errorf("mount point cannot be empty")
		}
		if mp[0] != '/' {
			return fmt.Errorf("mount point must be absolute path, got %s", mp)
		}
	}

	validLogLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "error": true, "fatal": true,
	}
	if !validLogLevels[c.Logging.Level] {
		return fmt.Errorf("invalid log level %s, must be one of: debug, info, warn, error, fatal", c.Logging.Level)
	}

	validFormats := map[string]bool{
		"json": true, "text": true,
	}
	if !validFormats[c.Logging.Format] {
		return fmt.Errorf("invalid log format %s, must be one of: json, text", c.Logging.Format)
	}

	return nil
}

// GetAddress returns the server address
func (c *Config) GetAddress() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// Clone returns a deep copy of the current configuration
func (c *Config) Clone() *Config {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return &Config{
		Server: ServerConfig{
			Host: c.Server.Host,
			Port: c.Server.Port,
			Path: c.Server.Path,
		},
		MountPoints: append([]string{}, c.MountPoints...),
		Interval:    c.Interval,
		Logging: LoggingConfig{
			Level:  c.Logging.Level,
			Format: c.Logging.Format,
		},
	}
}

// Update updates the configuration with new values atomically
func (c *Config) Update(newConfig *Config) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Server = newConfig.Server
	c.MountPoints = append([]string{}, newConfig.MountPoints...)
	c.Interval = newConfig.Interval
	c.Logging = newConfig.Logging
}

// ConfigWatcher watches for configuration file changes
type ConfigWatcher struct {
	configPath string
	config     *Config
	mu         sync.RWMutex
	callbacks  []func(*Config)
	running    bool
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewConfigWatcher creates a new configuration watcher
func NewConfigWatcher(configPath string, config *Config) *ConfigWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &ConfigWatcher{
		configPath: configPath,
		config:     config,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// AddCallback adds a callback function to be called when configuration changes
func (cw *ConfigWatcher) AddCallback(callback func(*Config)) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	cw.callbacks = append(cw.callbacks, callback)
}

// Watch starts watching for configuration file changes
func (cw *ConfigWatcher) Watch(interval time.Duration) error {
	cw.mu.Lock()
	if cw.running {
		cw.mu.Unlock()
		return fmt.Errorf("watcher is already running")
	}
	cw.running = true
	cw.mu.Unlock()

	go cw.watchLoop(interval)
	return nil
}

// watchLoop periodically checks for configuration file changes
func (cw *ConfigWatcher) watchLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var lastModTime time.Time

	// Get initial modification time
	if info, err := os.Stat(cw.configPath); err == nil {
		lastModTime = info.ModTime()
	}

	for {
		select {
		case <-cw.ctx.Done():
			return
		case <-ticker.C:
			info, err := os.Stat(cw.configPath)
			if err != nil {
				continue // File might not exist, skip
			}

			if info.ModTime().After(lastModTime) {
				lastModTime = info.ModTime()
				if err := cw.reloadConfig(); err != nil {
					// Log error but continue watching
					continue
				}
			}
		}
	}
}

// reloadConfig reloads the configuration from file
func (cw *ConfigWatcher) reloadConfig() error {
	newConfig, err := LoadFromFile(cw.configPath)
	if err != nil {
		return err
	}

	if err := newConfig.Validate(); err != nil {
		return err
	}

	// Update configuration atomically
	cw.mu.RLock()
	cw.config.Update(newConfig)

	// Call all callbacks
	for _, callback := range cw.callbacks {
		go callback(newConfig.Clone())
	}
	cw.mu.RUnlock()

	return nil
}

// Stop stops the configuration watcher
func (cw *ConfigWatcher) Stop() {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	if cw.running {
		cw.cancel()
		cw.running = false
	}
}

// GetConfig returns a copy of the current configuration
func (cw *ConfigWatcher) GetConfig() *Config {
	cw.mu.RLock()
	defer cw.mu.RUnlock()
	return cw.config.Clone()
}

// IsRunning returns whether the watcher is currently running
func (cw *ConfigWatcher) IsRunning() bool {
	cw.mu.RLock()
	defer cw.mu.RUnlock()
	return cw.running
}