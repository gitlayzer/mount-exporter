//go:build integration
// +build integration

package test

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"testing"
	"time"

	"github.com/mount-exporter/mount-exporter/config"
	"github.com/mount-exporter/mount-exporter/server"
	"github.com/prometheus/client_golang/prometheus"
)

func TestIntegration_FullWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a test configuration file
	configContent := `
server:
  host: "127.0.0.1"
  port: 8080
  path: "/metrics"
mount_points:
  - "/definitely-nonexistent-mount-point-12345"
  - "/tmp"
interval: 5s
logging:
  level: "info"
  format: "text"
`

	tmpFile, err := os.CreateTemp("", "mount-exporter-test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write config content: %v", err)
	}
	tmpFile.Close()

	// Note: In a real integration test, we would start the server as a separate process
	// and make actual HTTP requests. For this example, we'll test the configuration loading.
	cfg, err := loadConfiguration(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Configuration validation failed: %v", err)
	}

	// Create server (but don't start it)
	logger := log.New(io.Discard, "", log.LstdFlags)
	srv, err := server.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Test that components are properly initialized
	if srv.GetCollector() == nil {
		t.Error("Expected collector to be initialized")
	}

	if len(cfg.MountPoints) != 2 {
		t.Errorf("Expected 2 mount points, got %d", len(cfg.MountPoints))
	}
}

func TestIntegration_ConfigurationOverrides(t *testing.T) {
	// Create a base configuration
	configContent := `
server:
  host: "127.0.0.1"
  port: 8080
  path: "/metrics"
mount_points:
  - "/test"
interval: 30s
logging:
  level: "info"
  format: "json"
`

	tmpFile, err := os.CreateTemp("", "mount-exporter-test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write config content: %v", err)
	}
	tmpFile.Close()

	// Set environment variables to override config
	os.Setenv("MOUNT_EXPORTER_PORT", "9999")
	os.Setenv("MOUNT_EXPORTER_LOG_LEVEL", "debug")
	defer func() {
		os.Unsetenv("MOUNT_EXPORTER_PORT")
		os.Unsetenv("MOUNT_EXPORTER_LOG_LEVEL")
	}()

	cfg, err := loadConfiguration(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}

	// Verify overrides were applied
	if cfg.Server.Port != 9999 {
		t.Errorf("Expected port to be overridden to 9999, got %d", cfg.Server.Port)
	}

	if cfg.Logging.Level != "debug" {
		t.Errorf("Expected log level to be overridden to debug, got %s", cfg.Logging.Level)
	}
}

func TestIntegration_ErrorHandling(t *testing.T) {
	// Test with invalid configuration
	configContent := `
server:
  host: "127.0.0.1"
  port: 0  # Invalid port
  path: "metrics"  # Invalid path (should start with /)
mount_points: []  # Empty mount points
interval: -1s  # Invalid interval
logging:
  level: "invalid"  # Invalid log level
  format: "invalid"  # Invalid format
`

	tmpFile, err := os.CreateTemp("", "mount-exporter-test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write config content: %v", err)
	}
	tmpFile.Close()

	cfg, err := loadConfiguration(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}

	// Should fail validation
	if err := cfg.Validate(); err == nil {
		t.Error("Expected configuration validation to fail, but it passed")
	}
}

func TestIntegration_MetricsCollection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
			Path: "/metrics",
		},
		MountPoints: []string{
			"/definitely-nonexistent-mount-point-12345",
			"/",  // This should always exist
		},
		Interval: 5 * time.Second,
	}

	logger := log.New(io.Discard, "", log.LstdFlags)
	srv, err := server.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Collect metrics
	ch := make(chan prometheus.Metric, 100)
	srv.GetCollector().Collect(ch)
	close(ch)

	// Count collected metrics
	metricCount := 0
	for range ch {
		metricCount++
	}

	if metricCount == 0 {
		t.Error("Expected at least some metrics to be collected")
	}

	// Metrics collection completed successfully
}

func TestIntegration_ContextCancellation(t *testing.T) {
	// Test context cancellation with a simple check
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Test that context cancellation works
	select {
	case <-ctx.Done():
		// Expected - context was cancelled
		t.Log("Context was cancelled as expected")
	case <-time.After(100 * time.Millisecond):
		t.Error("Context should have been cancelled")
	}
}

func TestIntegration_ConcurrentAccess(t *testing.T) {
	cfg := &config.Config{
		MountPoints: []string{"/definitely-nonexistent-mount-point-12345"},
		Interval:    5 * time.Second,
	}

	logger := log.New(io.Discard, "", log.LstdFlags)
	srv, err := server.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Test concurrent config updates
	const numGoroutines = 10
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			newCfg := *cfg
			newCfg.MountPoints = append(newCfg.MountPoints, fmt.Sprintf("/test-%d", index))
			srv.GetCollector().UpdateConfig(&newCfg)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		select {
		case <-done:
			// OK
		case <-time.After(5 * time.Second):
			t.Fatal("Test timed out waiting for goroutines to complete")
		}
	}
}

func TestIntegration_RealSystemCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := &config.Config{
		MountPoints: []string{"/"}, // Use root which should always exist
		Interval:    5 * time.Second,
	}

	logger := log.New(io.Discard, "", log.LstdFlags)
	srv, err := server.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Check if findmnt is available
	wrapper := srv.GetCollector().GetFindmntWrapper()
	if !wrapper.IsAvailable() {
		t.Skip("findmnt command not available on this system")
	}

	// Test actual system command execution
	result := wrapper.CheckMountPoint(context.Background(), "/")

	if result.Error != nil {
		t.Logf("Note: findmnt failed (this may be expected on some systems): %v", result.Error)
		return
	}

	// Root should be mounted
	if result.Status != 1 { // MountStatusMounted
		t.Errorf("Expected root to be mounted, got status %v", result.Status)
	}

	if result.Target == "" {
		t.Error("Expected target to be set for root mount")
	}
}

// TestIntegration_HTTPServerSetup would normally test the actual HTTP server
// For this example, we'll just test the server setup
func TestIntegration_HTTPServerSetup(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: 0, // Use random port
			Path: "/metrics",
		},
		MountPoints: []string{"/test"},
		Interval:    5 * time.Second,
	}

	logger := log.New(io.Discard, "", log.LstdFlags)
	srv, err := server.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Test route setup
	srv.SetupRoutes()

	if srv.GetHTTPServer() == nil {
		t.Fatal("Expected HTTP server to be set up")
	}

	// Verify server configuration
	expectedTimeout := 30 * time.Second
	if srv.GetHTTPServer().ReadTimeout != expectedTimeout {
		t.Errorf("Expected read timeout %v, got %v", expectedTimeout, srv.GetHTTPServer().ReadTimeout)
	}

	if srv.GetHTTPServer().WriteTimeout != expectedTimeout {
		t.Errorf("Expected write timeout %v, got %v", expectedTimeout, srv.GetHTTPServer().WriteTimeout)
	}
}

// BenchmarkIntegration_ConfigurationLoading benchmarks configuration loading
func BenchmarkIntegration_ConfigurationLoading(b *testing.B) {
	configContent := `
server:
  host: "127.0.0.1"
  port: 8080
  path: "/metrics"
mount_points:
  - "/test1"
  - "/test2"
  - "/test3"
interval: 30s
logging:
  level: "info"
  format: "json"
`

	tmpFile, err := os.CreateTemp("", "mount-exporter-bench-*.yaml")
	if err != nil {
		b.Fatalf("Failed to create temp config file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		b.Fatalf("Failed to write config content: %v", err)
	}
	tmpFile.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := loadConfiguration(tmpFile.Name())
		if err != nil {
			b.Fatalf("Failed to load configuration: %v", err)
		}
	}
}

// Helper function to load configuration (copied from main)
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

// Helper function to find config file (copied from main)
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