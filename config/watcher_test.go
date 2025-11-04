package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestConfigWatcher(t *testing.T) {
	// Create temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Initial config
	initialConfig := &Config{
		Server: ServerConfig{
			Host: "localhost",
			Port: 8080,
			Path: "/metrics",
		},
		MountPoints: []string{"/test"},
		Interval:    30 * time.Second,
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	data, err := yaml.Marshal(initialConfig)
	if err != nil {
		t.Fatalf("Failed to marshal initial config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to write initial config: %v", err)
	}

	// Load config and create watcher
	config, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	watcher := NewConfigWatcher(configPath, config)

	// Test callback
	callbackCalled := false
	var callbackConfig *Config

	watcher.AddCallback(func(newConfig *Config) {
		callbackCalled = true
		callbackConfig = newConfig
	})

	// Start watching
	if err := watcher.Watch(100 * time.Millisecond); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}

	// Verify watcher is running
	if !watcher.IsRunning() {
		t.Error("Watcher should be running")
	}

	// Update config file
	updatedConfig := *initialConfig
	updatedConfig.Server.Port = 9090
	updatedConfig.MountPoints = []string{"/test1", "/test2"}

	data, err = yaml.Marshal(updatedConfig)
	if err != nil {
		t.Fatalf("Failed to marshal updated config: %v", err)
	}

	// Wait a bit to ensure different modification time
	time.Sleep(10 * time.Millisecond)

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to write updated config: %v", err)
	}

	// Wait for callback
	select {
	case <-time.After(2 * time.Second):
		t.Fatal("Callback was not called within timeout")
	default:
		time.Sleep(500 * time.Millisecond) // Give time for watcher to detect change
		if !callbackCalled {
			t.Error("Callback was not called")
		}
	}

	// Verify callback config
	if callbackConfig == nil {
		t.Fatal("Callback config is nil")
	}

	if callbackConfig.Server.Port != 9090 {
		t.Errorf("Expected port 9090, got %d", callbackConfig.Server.Port)
	}

	if len(callbackConfig.MountPoints) != 2 {
		t.Errorf("Expected 2 mount points, got %d", len(callbackConfig.MountPoints))
	}

	// Stop watcher
	watcher.Stop()

	// Verify watcher is stopped
	if watcher.IsRunning() {
		t.Error("Watcher should be stopped")
	}
}

func TestConfigClone(t *testing.T) {
	original := &Config{
		Server: ServerConfig{
			Host: "localhost",
			Port: 8080,
			Path: "/metrics",
		},
		MountPoints: []string{"/test1", "/test2"},
		Interval:    30 * time.Second,
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	cloned := original.Clone()

	// Verify values are the same
	if cloned.Server.Host != original.Server.Host {
		t.Errorf("Host mismatch: expected %s, got %s", original.Server.Host, cloned.Server.Host)
	}

	if len(cloned.MountPoints) != len(original.MountPoints) {
		t.Errorf("Mount points length mismatch: expected %d, got %d", len(original.MountPoints), len(cloned.MountPoints))
	}

	// Verify it's a deep copy (modifying clone doesn't affect original)
	cloned.Server.Port = 9090
	cloned.MountPoints[0] = "/modified"

	if original.Server.Port == 9090 {
		t.Error("Original should not be affected by clone modification")
	}

	if original.MountPoints[0] == "/modified" {
		t.Error("Original mount points should not be affected by clone modification")
	}
}

func TestConfigUpdate(t *testing.T) {
	config := &Config{
		Server: ServerConfig{
			Host: "localhost",
			Port: 8080,
			Path: "/metrics",
		},
		MountPoints: []string{"/test1"},
		Interval:    30 * time.Second,
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	newConfig := &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 9090,
			Path: "/admin",
		},
		MountPoints: []string{"/test2", "/test3"},
		Interval:    60 * time.Second,
		Logging: LoggingConfig{
			Level:  "debug",
			Format: "text",
		},
	}

	config.Update(newConfig)

	// Verify values were updated
	if config.Server.Host != "0.0.0.0" {
		t.Errorf("Expected Host to be updated to 0.0.0.0, got %s", config.Server.Host)
	}

	if config.Server.Port != 9090 {
		t.Errorf("Expected Port to be updated to 9090, got %d", config.Server.Port)
	}

	if len(config.MountPoints) != 2 {
		t.Errorf("Expected 2 mount points, got %d", len(config.MountPoints))
	}

	if config.Interval != 60*time.Second {
		t.Errorf("Expected Interval to be updated to 60s, got %v", config.Interval)
	}

	if config.Logging.Level != "debug" {
		t.Errorf("Expected Level to be updated to debug, got %s", config.Logging.Level)
	}
}

func TestConfigWatcher_DoubleStart(t *testing.T) {
	config := &Config{}
	watcher := NewConfigWatcher("/tmp/test.yaml", config)

	if err := watcher.Watch(100 * time.Millisecond); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}

	// Try to start again
	if err := watcher.Watch(100 * time.Millisecond); err == nil {
		t.Error("Expected error when starting watcher twice")
	}

	watcher.Stop()
}

func TestConfigWatcher_GetConfig(t *testing.T) {
	config := &Config{
		Server: ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
		MountPoints: []string{"/test"},
	}

	watcher := NewConfigWatcher("/tmp/test.yaml", config)

	retrievedConfig := watcher.GetConfig()

	// Verify it's a copy (modifying retrieved config doesn't affect original)
	retrievedConfig.Server.Port = 9090

	if config.Server.Port == 9090 {
		t.Error("Original config should not be affected by retrieved config modification")
	}
}

func TestConfigWatcher_InvalidConfigReload(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Initial valid config
	initialConfig := &Config{
		Server: ServerConfig{
			Host: "localhost",
			Port: 8080,
			Path: "/metrics",
		},
		MountPoints: []string{"/test"},
		Interval:    30 * time.Second,
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	data, err := yaml.Marshal(initialConfig)
	if err != nil {
		t.Fatalf("Failed to marshal initial config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to write initial config: %v", err)
	}

	config, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	watcher := NewConfigWatcher(configPath, config)
	callbackCalled := false

	watcher.AddCallback(func(newConfig *Config) {
		callbackCalled = true
	})

	if err := watcher.Watch(100 * time.Millisecond); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}

	// Write invalid config (port out of range)
	invalidConfig := *initialConfig
	invalidConfig.Server.Port = 99999

	data, err = yaml.Marshal(invalidConfig)
	if err != nil {
		t.Fatalf("Failed to marshal invalid config: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to write invalid config: %v", err)
	}

	// Wait and verify callback was not called due to validation error
	time.Sleep(500 * time.Millisecond)
	if callbackCalled {
		t.Error("Callback should not be called for invalid config")
	}

	watcher.Stop()
}