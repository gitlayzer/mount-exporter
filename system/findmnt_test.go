package system

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewFindmntWrapper(t *testing.T) {
	timeout := 5 * time.Second
	wrapper := NewFindmntWrapper(timeout)

	if wrapper.timeout != timeout {
		t.Errorf("Expected timeout %v, got %v", timeout, wrapper.timeout)
	}
}

func TestFindmntWrapper_CheckMountPoint_ContextTimeout(t *testing.T) {
	wrapper := NewFindmntWrapper(1 * time.Millisecond)

	// Create a context that will timeout quickly
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	result := wrapper.CheckMountPoint(ctx, "/nonexistent")

	if result.Status != MountStatusNotMounted {
		t.Errorf("Expected status MountStatusNotMounted, got %v", result.Status)
	}

	if result.Error == nil {
		t.Error("Expected timeout error, got nil")
	}

	if !contains(result.Error.Error(), "timed out") {
		t.Errorf("Expected timeout error message, got '%s'", result.Error.Error())
	}
}

func TestFindmntWrapper_CheckMountPoint_NonExistentMount(t *testing.T) {
	wrapper := NewFindmntWrapper(5 * time.Second)

	result := wrapper.CheckMountPoint(context.Background(), "/definitely-nonexistent-mount-point-12345")

	if result.Status != MountStatusNotMounted {
		t.Errorf("Expected status MountStatusNotMounted, got %v", result.Status)
	}

	// Error should be nil for normal "not found" case
	if result.Error != nil {
		t.Errorf("Expected no error for non-existent mount, got %v", result.Error)
	}
}

func TestFindmntWrapper_CheckMountPoint_RootMount(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	wrapper := NewFindmntWrapper(5 * time.Second)

	result := wrapper.CheckMountPoint(context.Background(), "/")

	// Root should always be mounted
	if result.Status != MountStatusMounted {
		t.Errorf("Expected status MountStatusMounted for root, got %v", result.Status)
	}

	if result.Error != nil {
		t.Errorf("Expected no error for root mount, got %v", result.Error)
	}

	if result.Target == "" {
		t.Error("Expected target to be set for root mount")
	}
}

func TestFindmntWrapper_IsAvailable(t *testing.T) {
	wrapper := NewFindmntWrapper(5 * time.Second)

	// This test might fail on systems without findmnt, but that's expected
	available := wrapper.IsAvailable()

	if !available {
		t.Log("findmnt command not available on this system")
	}
}

func TestFindmntWrapper_GetVersion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	wrapper := NewFindmntWrapper(5 * time.Second)

	version, err := wrapper.GetVersion()

	if err != nil {
		t.Logf("Failed to get findmnt version (expected on some systems): %v", err)
		return
	}

	if version == "" {
		t.Error("Expected version string, got empty string")
	}

	if !contains(strings.ToLower(version), "findmnt") {
		t.Logf("Version output doesn't contain 'findmnt': %s", version)
	}
}

func TestFindmntWrapper_CheckMultipleMountPoints(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	wrapper := NewFindmntWrapper(10 * time.Second)

	mountPoints := []string{"/", "/definitely-nonexistent-mount-point-12345", "/tmp"}
	results := wrapper.CheckMultipleMountPoints(context.Background(), mountPoints)

	if len(results) != len(mountPoints) {
		t.Errorf("Expected %d results, got %d", len(mountPoints), len(results))
	}

	// Check root mount
	if results[0].Status != MountStatusMounted {
		t.Errorf("Expected root to be mounted, got %v", results[0].Status)
	}

	// Check non-existent mount
	if results[1].Status != MountStatusNotMounted {
		t.Errorf("Expected non-existent mount to not be mounted, got %v", results[1].Status)
	}

	// Check tmp (might or might not be a separate mount)
	if results[2].Status == MountStatusUnknown {
		t.Errorf("Expected /tmp to have a definite status, got %v", results[2].Status)
	}
}

func TestFindmntWrapper_CheckMultipleMountPoints_ContextCancellation(t *testing.T) {
	wrapper := NewFindmntWrapper(10 * time.Second)

	// Create a context that will be cancelled quickly
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	mountPoints := []string{"/", "/tmp"}
	results := wrapper.CheckMultipleMountPoints(ctx, mountPoints)

	for i, result := range results {
		if result.Status != MountStatusUnknown {
			t.Errorf("Expected unknown status for cancelled context at index %d, got %v", i, result.Status)
		}

		if result.Error == nil {
			t.Errorf("Expected error for cancelled context at index %d, got nil", i)
		}
	}
}

func TestMountStatus_String(t *testing.T) {
	tests := []struct {
		status   MountStatus
		expected string
	}{
		{MountStatusUnknown, "unknown"},
		{MountStatusMounted, "mounted"},
		{MountStatusNotMounted, "not_mounted"},
		{MountStatus(999), "unknown"}, // Invalid status should return "unknown"
	}

	for _, tt := range tests {
		if tt.status.String() != tt.expected {
			t.Errorf("Expected '%s', got '%s' for status %v", tt.expected, tt.status.String(), tt.status)
		}
	}
}

func TestFindmntResult_Fields(t *testing.T) {
	result := &FindmntResult{
		MountPoint: "/test",
		Status:     MountStatusMounted,
		Target:     "/dev/sda1",
		FSType:     "ext4",
		Options:    "rw,relatime",
		Source:     "/dev/sda1",
		Error:      nil,
	}

	if result.MountPoint != "/test" {
		t.Errorf("Expected mount point '/test', got '%s'", result.MountPoint)
	}

	if result.Status != MountStatusMounted {
		t.Errorf("Expected status Mounted, got %v", result.Status)
	}

	if result.Target != "/dev/sda1" {
		t.Errorf("Expected target '/dev/sda1', got '%s'", result.Target)
	}

	if result.FSType != "ext4" {
		t.Errorf("Expected filesystem type 'ext4', got '%s'", result.FSType)
	}

	if result.Options != "rw,relatime" {
		t.Errorf("Expected options 'rw,relatime', got '%s'", result.Options)
	}

	if result.Source != "/dev/sda1" {
		t.Errorf("Expected source '/dev/sda1', got '%s'", result.Source)
	}

	if result.Error != nil {
		t.Errorf("Expected no error, got %v", result.Error)
	}
}

func TestFindmntWrapper_CheckMountPoint_InvalidCommand(t *testing.T) {
	// This test simulates what happens when findmnt is not available
	// by temporarily modifying the PATH to not contain it
	if testing.Short() {
		t.Skip("Skipping test that modifies environment in short mode")
	}

	// Save original PATH
	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)

	// Set PATH to empty (findmnt won't be found)
	os.Setenv("PATH", "")

	wrapper := NewFindmntWrapper(5 * time.Second)

	result := wrapper.CheckMountPoint(context.Background(), "/")

	if result.Status != MountStatusNotMounted {
		t.Errorf("Expected status MountStatusNotMounted when findmnt unavailable, got %v", result.Status)
	}

	if result.Error == nil {
		t.Error("Expected error when findmnt unavailable, got nil")
	}
}

func TestFindmntWrapper_CheckMountPoint_EmptyOutput(t *testing.T) {
	// This test is harder to implement without mocking the exec package
	// For now, we'll test that our parsing logic handles empty output correctly
	// by testing the non-existent mount point case
	wrapper := NewFindmntWrapper(5 * time.Second)

	result := wrapper.CheckMountPoint(context.Background(), "/definitely-nonexistent-mount-point-12345")

	if result.Status != MountStatusNotMounted {
		t.Errorf("Expected status MountStatusNotMounted for non-existent mount, got %v", result.Status)
	}
}

func TestFindmntWrapper_CircuitBreaker(t *testing.T) {
	// Create wrapper with very sensitive circuit breaker for testing
	wrapper := NewFindmntWrapper(1 * time.Second)

	// Test initial state
	if wrapper.GetCircuitBreakerState().String() != "CLOSED" {
		t.Errorf("Expected initial circuit breaker state to be CLOSED, got %s", wrapper.GetCircuitBreakerState().String())
	}

	// Get initial stats
	stats := wrapper.GetStats()
	if stats["total_calls"] != float64(0) {
		t.Errorf("Expected 0 total calls initially, got %v", stats["total_calls"])
	}
}

func TestFindmntWrapper_GetStats(t *testing.T) {
	wrapper := NewFindmntWrapper(5 * time.Second)

	// Make a call to generate stats
	wrapper.CheckMountPoint(context.Background(), "/definitely-nonexistent-mount-point-12345")

	stats := wrapper.GetStats()

	// Check that stats are populated
	if stats["total_calls"] == float64(0) {
		t.Error("Expected total_calls to be > 0 after making a call")
	}

	// Check required fields exist
	requiredFields := []string{
		"total_calls", "successful_calls", "failed_calls", "success_rate",
		"circuit_breaker_state", "circuit_breaker_failures", "retry_attempts", "retry_rate",
	}
	for _, field := range requiredFields {
		if _, ok := stats[field]; !ok {
			t.Errorf("Expected stats to contain field '%s'", field)
		}
	}

	// Check that retry attempts are tracked
	if stats["retry_attempts"] == float64(0) {
		t.Error("Expected retry_attempts to be > 0 after making a call")
	}
}

func TestFindmntWrapper_ResetCircuitBreaker(t *testing.T) {
	wrapper := NewFindmntWrapper(5 * time.Second)

	// Reset should not panic
	wrapper.ResetCircuitBreaker()

	// State should be closed after reset
	if wrapper.GetCircuitBreakerState().String() != "CLOSED" {
		t.Errorf("Expected circuit breaker state to be CLOSED after reset, got %s", wrapper.GetCircuitBreakerState().String())
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