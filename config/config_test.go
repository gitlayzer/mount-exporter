package config

import (
	"os"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Server.Host != "0.0.0.0" {
		t.Errorf("Expected default host '0.0.0.0', got '%s'", config.Server.Host)
	}

	if config.Server.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", config.Server.Port)
	}

	if config.Server.Path != "/metrics" {
		t.Errorf("Expected default path '/metrics', got '%s'", config.Server.Path)
	}

	if config.Interval != 30*time.Second {
		t.Errorf("Expected default interval 30s, got %v", config.Interval)
	}

	if config.Logging.Level != "info" {
		t.Errorf("Expected default log level 'info', got '%s'", config.Logging.Level)
	}

	if config.Logging.Format != "json" {
		t.Errorf("Expected default log format 'json', got '%s'", config.Logging.Format)
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create a temporary config file
	configContent := `
server:
  host: "127.0.0.1"
  port: 9090
  path: "/custom-metrics"
mount_points:
  - "/test1"
  - "/test2"
interval: 60s
logging:
  level: "debug"
  format: "text"
`

	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write config content: %v", err)
	}
	tmpFile.Close()

	// Load the config
	config, err := LoadFromFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify loaded values
	if config.Server.Host != "127.0.0.1" {
		t.Errorf("Expected host '127.0.0.1', got '%s'", config.Server.Host)
	}

	if config.Server.Port != 9090 {
		t.Errorf("Expected port 9090, got %d", config.Server.Port)
	}

	if config.Server.Path != "/custom-metrics" {
		t.Errorf("Expected path '/custom-metrics', got '%s'", config.Server.Path)
	}

	if len(config.MountPoints) != 2 {
		t.Errorf("Expected 2 mount points, got %d", len(config.MountPoints))
	}

	if config.MountPoints[0] != "/test1" {
		t.Errorf("Expected first mount point '/test1', got '%s'", config.MountPoints[0])
	}

	if config.Interval != 60*time.Second {
		t.Errorf("Expected interval 60s, got %v", config.Interval)
	}

	if config.Logging.Level != "debug" {
		t.Errorf("Expected log level 'debug', got '%s'", config.Logging.Level)
	}

	if config.Logging.Format != "text" {
		t.Errorf("Expected log format 'text', got '%s'", config.Logging.Format)
	}
}

func TestLoadFromFile_NonExistent(t *testing.T) {
	config, err := LoadFromFile("non-existent-file.yaml")
	if err != nil {
		t.Fatalf("Should not error for non-existent file: %v", err)
	}

	// Should return default config
	if config.Server.Port != 8080 {
		t.Errorf("Expected default config for non-existent file, got port %d", config.Server.Port)
	}
}

func TestLoadFromFile_InvalidYAML(t *testing.T) {
	invalidContent := `
server:
  host: "localhost"
  port: invalid_port_number
path: "/metrics"
`

	tmpFile, err := os.CreateTemp("", "invalid-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(invalidContent); err != nil {
		t.Fatalf("Failed to write config content: %v", err)
	}
	tmpFile.Close()

	_, err = LoadFromFile(tmpFile.Name())
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}

func TestApplyEnvOverrides(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected func(*Config)
	}{
		{
			name: "Override all server settings",
			envVars: map[string]string{
				"MOUNT_EXPORTER_HOST":     "192.168.1.1",
				"MOUNT_EXPORTER_PORT":     "9999",
				"MOUNT_EXPORTER_PATH":     "/custom",
				"MOUNT_EXPORTER_INTERVAL": "120s",
				"MOUNT_EXPORTER_LOG_LEVEL": "warn",
			},
			expected: func(c *Config) {
				if c.Server.Host != "192.168.1.1" {
					t.Errorf("Expected host '192.168.1.1', got '%s'", c.Server.Host)
				}
				if c.Server.Port != 9999 {
					t.Errorf("Expected port 9999, got %d", c.Server.Port)
				}
				if c.Server.Path != "/custom" {
					t.Errorf("Expected path '/custom', got '%s'", c.Server.Path)
				}
				if c.Interval != 120*time.Second {
					t.Errorf("Expected interval 120s, got %v", c.Interval)
				}
				if c.Logging.Level != "warn" {
					t.Errorf("Expected log level 'warn', got '%s'", c.Logging.Level)
				}
			},
		},
		{
			name: "Invalid port number",
			envVars: map[string]string{
				"MOUNT_EXPORTER_PORT": "invalid",
			},
			expected: func(c *Config) {
				// Should not be applied
				if c.Server.Port != 8080 {
					t.Errorf("Expected default port 8080, got %d", c.Server.Port)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}
			defer func() {
				// Clean up environment variables
				for key := range tt.envVars {
					os.Unsetenv(key)
				}
			}()

			config := DefaultConfig()
			err := config.applyEnvOverrides()

			if tt.name == "Invalid port number" {
				if err == nil {
					t.Error("Expected error for invalid port number, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			tt.expected(config)
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid config",
			config: &Config{
				Server: ServerConfig{
					Host: "0.0.0.0",
					Port: 8080,
					Path: "/metrics",
				},
				MountPoints: []string{"/data", "/var"},
				Interval:    30 * time.Second,
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
			},
			wantErr: false,
		},
		{
			name: "Invalid port",
			config: &Config{
				Server: ServerConfig{
					Host: "0.0.0.0",
					Port: 0,
					Path: "/metrics",
				},
				MountPoints: []string{"/data"},
				Interval:    30 * time.Second,
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
			},
			wantErr: true,
			errMsg:  "server port must be between 1 and 65535",
		},
		{
			name: "Invalid path",
			config: &Config{
				Server: ServerConfig{
					Host: "0.0.0.0",
					Port: 8080,
					Path: "metrics",
				},
				MountPoints: []string{"/data"},
				Interval:    30 * time.Second,
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
			},
			wantErr: true,
			errMsg:  "server path must start with '/'",
		},
		{
			name: "Empty mount points",
			config: &Config{
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
			},
			wantErr: true,
			errMsg:  "at least one mount point must be configured",
		},
		{
			name: "Relative mount point path",
			config: &Config{
				Server: ServerConfig{
					Host: "0.0.0.0",
					Port: 8080,
					Path: "/metrics",
				},
				MountPoints: []string{"relative/path"},
				Interval:    30 * time.Second,
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
			},
			wantErr: true,
			errMsg:  "mount point must be absolute path",
		},
		{
			name: "Invalid log level",
			config: &Config{
				Server: ServerConfig{
					Host: "0.0.0.0",
					Port: 8080,
					Path: "/metrics",
				},
				MountPoints: []string{"/data"},
				Interval:    30 * time.Second,
				Logging: LoggingConfig{
					Level:  "invalid",
					Format: "json",
				},
			},
			wantErr: true,
			errMsg:  "invalid log level",
		},
		{
			name: "Invalid log format",
			config: &Config{
				Server: ServerConfig{
					Host: "0.0.0.0",
					Port: 8080,
					Path: "/metrics",
				},
				MountPoints: []string{"/data"},
				Interval:    30 * time.Second,
				Logging: LoggingConfig{
					Level:  "info",
					Format: "invalid",
				},
			},
			wantErr: true,
			errMsg:  "invalid log format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil {
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error message to contain '%s', got '%s'", tt.errMsg, err.Error())
				}
			}
		})
	}
}

func TestGetAddress(t *testing.T) {
	config := &Config{
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 9090,
		},
	}

	address := config.GetAddress()
	expected := "127.0.0.1:9090"
	if address != expected {
		t.Errorf("Expected address '%s', got '%s'", expected, address)
	}
}

func TestLoadFromFile_EmptyFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "empty-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	config, err := LoadFromFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load empty config file: %v", err)
	}

	// Should return default config
	if config.Server.Port != 8080 {
		t.Errorf("Expected default config for empty file, got port %d", config.Server.Port)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		func() bool {
			for i := 1; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())))
}