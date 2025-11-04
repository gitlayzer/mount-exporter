package resources

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestLogger implements Logger interface for testing
type TestLogger struct {
	messages []string
	mu       sync.Mutex
}

func (l *TestLogger) Printf(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messages = append(l.messages, fmt.Sprintf(format, args...))
}

func (l *TestLogger) GetMessages() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string{}, l.messages...)
}

func (l *TestLogger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messages = nil
}

func TestNewResourceManager(t *testing.T) {
	logger := &TestLogger{}
	rm := NewResourceManager(ResourceManagerConfig{
		Logger:    logger,
		EnableGC:  false,
		GCInterval: 1 * time.Minute,
	})

	stats := rm.GetStats()
	if stats["total_resources"] != int64(0) {
		t.Errorf("Expected 0 total resources initially, got %v", stats["total_resources"])
	}

	if stats["active_resources"] != int64(0) {
		t.Errorf("Expected 0 active resources initially, got %v", stats["active_resources"])
	}
}

func TestResourceManager_RegisterResource(t *testing.T) {
	logger := &TestLogger{}
	rm := NewResourceManager(ResourceManagerConfig{
		Logger:   logger,
		EnableGC: false,
	})

	// Register a resource
	cleanupCalled := false
	rm.RegisterResource("test-resource", ResourceTypeFile, "Test file resource", func() error {
		cleanupCalled = true
		return nil
	})

	// Check that resource was registered
	resource, exists := rm.GetResource("test-resource")
	if !exists {
		t.Error("Expected resource to be registered")
	}

	if resource.ID != "test-resource" {
		t.Errorf("Expected resource ID 'test-resource', got %s", resource.ID)
	}

	if resource.Type != ResourceTypeFile {
		t.Errorf("Expected resource type File, got %s", resource.Type.String())
	}

	// Check stats
	stats := rm.GetStats()
	if stats["total_resources"] != int64(1) {
		t.Errorf("Expected 1 total resource, got %v", stats["total_resources"])
	}

	if stats["active_resources"] != int64(1) {
		t.Errorf("Expected 1 active resource, got %v", stats["active_resources"])
	}

	// Check log message
	messages := logger.GetMessages()
	if len(messages) == 0 {
		t.Error("Expected resource registration to be logged")
	}
}

func TestResourceManager_UnregisterResource(t *testing.T) {
	logger := &TestLogger{}
	rm := NewResourceManager(ResourceManagerConfig{
		Logger:   logger,
		EnableGC: false,
	})

	// Register a resource
	cleanupCalled := false
	rm.RegisterResource("test-resource", ResourceTypeFile, "Test file resource", func() error {
		cleanupCalled = true
		return nil
	})

	// Unregister the resource
	err := rm.UnregisterResource("test-resource")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !cleanupCalled {
		t.Error("Expected cleanup function to be called")
	}

	// Check that resource was unregistered
	_, exists := rm.GetResource("test-resource")
	if exists {
		t.Error("Expected resource to be unregistered")
	}

	// Check stats
	stats := rm.GetStats()
	if stats["active_resources"] != int64(0) {
		t.Errorf("Expected 0 active resources after unregister, got %v", stats["active_resources"])
	}

	if stats["cleaned_resources"] != int64(1) {
		t.Errorf("Expected 1 cleaned resource, got %v", stats["cleaned_resources"])
	}
}

func TestResourceManager_UnregisterResource_NotFound(t *testing.T) {
	logger := &TestLogger{}
	rm := NewResourceManager(ResourceManagerConfig{
		Logger:   logger,
		EnableGC: false,
	})

	// Try to unregister non-existent resource
	err := rm.UnregisterResource("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent resource")
	}

	if err.Error() != "resource non-existent not found" {
		t.Errorf("Expected 'resource non-existent not found', got %s", err.Error())
	}
}

func TestResourceManager_CleanupResource(t *testing.T) {
	logger := &TestLogger{}
	rm := NewResourceManager(ResourceManagerConfig{
		Logger:   logger,
		EnableGC: false,
	})

	// Register a resource
	cleanupCalled := false
	rm.RegisterResource("test-resource", ResourceTypeFile, "Test file resource", func() error {
		cleanupCalled = true
		return nil
	})

	// Cleanup the resource (without unregistering)
	err := rm.CleanupResource("test-resource")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !cleanupCalled {
		t.Error("Expected cleanup function to be called")
	}

	// Resource should still be registered
	_, exists := rm.GetResource("test-resource")
	if !exists {
		t.Error("Expected resource to still be registered after cleanup")
	}
}

func TestResourceManager_CleanupAll(t *testing.T) {
	logger := &TestLogger{}
	rm := NewResourceManager(ResourceManagerConfig{
		Logger:   logger,
		EnableGC: false,
	})

	// Register multiple resources
	cleanupCount := 0
	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("resource-%d", i)
		rm.RegisterResource(id, ResourceTypeFile, fmt.Sprintf("Test resource %d", i), func() error {
			cleanupCount++
			return nil
		})
	}

	// Cleanup all resources
	errors := rm.CleanupAll()
	if len(errors) > 0 {
		t.Errorf("Expected no cleanup errors, got %v", errors)
	}

	if cleanupCount != 3 {
		t.Errorf("Expected 3 cleanup calls, got %d", cleanupCount)
	}

	// All resources should be unregistered
	resources := rm.ListResources()
	if len(resources) != 0 {
		t.Errorf("Expected 0 resources after cleanup all, got %d", len(resources))
	}

	// Check stats
	stats := rm.GetStats()
	if stats["active_resources"] != int64(0) {
		t.Errorf("Expected 0 active resources after cleanup all, got %v", stats["active_resources"])
	}

	if stats["cleaned_resources"] != int64(3) {
		t.Errorf("Expected 3 cleaned resources, got %v", stats["cleaned_resources"])
	}
}

func TestResourceManager_CleanupAll_WithErrors(t *testing.T) {
	logger := &TestLogger{}
	rm := NewResourceManager(ResourceManagerConfig{
		Logger:   logger,
		EnableGC: false,
	})

	// Register resources with cleanup errors
	rm.RegisterResource("resource-1", ResourceTypeFile, "Test resource 1", func() error {
		return nil // Success
	})

	rm.RegisterResource("resource-2", ResourceTypeFile, "Test resource 2", func() error {
		return fmt.Errorf("cleanup failed") // Failure
	})

	rm.RegisterResource("resource-3", ResourceTypeFile, "Test resource 3", func() error {
		return nil // Success
	})

	// Cleanup all resources
	errors := rm.CleanupAll()
	if len(errors) != 1 {
		t.Errorf("Expected 1 cleanup error, got %d", len(errors))
	}

	// Check stats
	stats := rm.GetStats()
	if stats["failed_cleanups"] != int64(1) {
		t.Errorf("Expected 1 failed cleanup, got %v", stats["failed_cleanups"])
	}

	if stats["cleaned_resources"] != int64(2) {
		t.Errorf("Expected 2 cleaned resources, got %v", stats["cleaned_resources"])
	}
}

func TestResourceManager_ListResources(t *testing.T) {
	logger := &TestLogger{}
	rm := NewResourceManager(ResourceManagerConfig{
		Logger:   logger,
		EnableGC: false,
	})

	// Register multiple resources
	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("resource-%d", i)
		rm.RegisterResource(id, ResourceTypeFile, fmt.Sprintf("Test resource %d", i), nil)
	}

	// List resources
	resources := rm.ListResources()
	if len(resources) != 3 {
		t.Errorf("Expected 3 resources, got %d", len(resources))
	}

	// Check that all resources are present
	resourceMap := make(map[string]bool)
	for _, resource := range resources {
		resourceMap[resource.ID] = true
	}

	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("resource-%d", i)
		if !resourceMap[id] {
			t.Errorf("Expected resource %s to be in list", id)
		}
	}
}

func TestResourceManager_RunGC(t *testing.T) {
	logger := &TestLogger{}
	rm := NewResourceManager(ResourceManagerConfig{
		Logger:   logger,
		EnableGC: false,
	})

	// Run GC
	rm.RunGC()

	// Check that GC was run (stats should be updated)
	stats := rm.GetStats()
	if stats["last_gc"] == nil {
		t.Error("Expected last_gc to be set after RunGC")
	}

	// Check log message
	messages := logger.GetMessages()
	if len(messages) == 0 {
		t.Error("Expected GC to be logged")
	}

	found := false
	for _, msg := range messages {
		if contains(msg, "Forced garbage collection") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'Forced garbage collection' in log messages")
	}
}

func TestResourceManager_Close(t *testing.T) {
	logger := &TestLogger{}
	rm := NewResourceManager(ResourceManagerConfig{
		Logger:   logger,
		EnableGC: false,
	})

	// Register a resource
	cleanupCalled := false
	rm.RegisterResource("test-resource", ResourceTypeFile, "Test file resource", func() error {
		cleanupCalled = true
		return nil
	})

	// Close resource manager
	rm.Close()

	if !cleanupCalled {
		t.Error("Expected cleanup function to be called on close")
	}

	// Check log messages
	messages := logger.GetMessages()
	if len(messages) == 0 {
		t.Error("Expected close to be logged")
	}

	found := false
	for _, msg := range messages {
		if contains(msg, "Shutting down resource manager") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected shutdown message in log")
	}
}

func TestResourceManager_GetStats(t *testing.T) {
	logger := &TestLogger{}
	rm := NewResourceManager(ResourceManagerConfig{
		Logger:   logger,
		EnableGC: false,
	})

	// Register a resource
	rm.RegisterResource("test-resource", ResourceTypeFile, "Test file resource", nil)

	// Get stats
	stats := rm.GetStats()

	// Check required fields
	requiredFields := []string{
		"total_resources", "active_resources", "cleaned_resources",
		"failed_cleanups", "memory_usage_mb", "goroutine_count", "last_gc",
	}

	for _, field := range requiredFields {
		if _, ok := stats[field]; !ok {
			t.Errorf("Expected stats to contain field '%s'", field)
		}
	}

	// Check values
	if stats["total_resources"] != int64(1) {
		t.Errorf("Expected 1 total resource, got %v", stats["total_resources"])
	}

	if stats["active_resources"] != int64(1) {
		t.Errorf("Expected 1 active resource, got %v", stats["active_resources"])
	}

	if stats["memory_usage_mb"].(float64) < 0 {
		t.Error("Expected memory usage to be non-negative")
	}

	if stats["goroutine_count"].(int64) <= 0 {
		t.Error("Expected goroutine count to be positive")
	}
}

func TestResourceType_String(t *testing.T) {
	tests := map[ResourceType]string{
		ResourceTypeFile:       "File",
		ResourceTypeNetwork:    "Network",
		ResourceTypeMemory:     "Memory",
		ResourceTypeGoroutine:  "Goroutine",
		ResourceTypeCustom:     "Custom",
		ResourceType(999):      "Unknown",
	}

	for resourceType, expected := range tests {
		if resourceType.String() != expected {
			t.Errorf("Expected '%s', got '%s' for type %v", expected, resourceType.String(), resourceType)
		}
	}
}

func TestWithCleanup(t *testing.T) {
	logger := &TestLogger{}
	rm := NewResourceManager(ResourceManagerConfig{
		Logger:   logger,
		EnableGC: false,
	})

	// Use WithCleanup helper
	cleanupCalled := false
	setupFunc := WithCleanup("test-resource", ResourceTypeCustom, "Test custom resource", func() error {
		cleanupCalled = true
		return nil
	})

	setupFunc(rm)

	// Check that resource was registered
	_, exists := rm.GetResource("test-resource")
	if !exists {
		t.Error("Expected resource to be registered")
	}

	// Cleanup and check
	rm.UnregisterResource("test-resource")
	if !cleanupCalled {
		t.Error("Expected cleanup function to be called")
	}
}

func TestNewFileResource(t *testing.T) {
	logger := &TestLogger{}
	rm := NewResourceManager(ResourceManagerConfig{
		Logger:   logger,
		EnableGC: false,
	})

	// Use NewFileResource helper
	cleanupCalled := false
	setupFunc := NewFileResource("test-file", "/tmp/test.txt", func() error {
		cleanupCalled = true
		return nil
	})

	setupFunc(rm)

	// Check that resource was registered with correct type
	resource, exists := rm.GetResource("test-file")
	if !exists {
		t.Error("Expected resource to be registered")
	}

	if resource.Type != ResourceTypeFile {
		t.Errorf("Expected resource type File, got %s", resource.Type.String())
	}

	if !contains(resource.Description, "/tmp/test.txt") {
		t.Errorf("Expected description to contain file path, got %s", resource.Description)
	}
}

func TestNewNetworkResource(t *testing.T) {
	logger := &TestLogger{}
	rm := NewResourceManager(ResourceManagerConfig{
		Logger:   logger,
		EnableGC: false,
	})

	// Use NewNetworkResource helper
	cleanupCalled := false
	setupFunc := NewNetworkResource("test-conn", "Database connection", func() error {
		cleanupCalled = true
		return nil
	})

	setupFunc(rm)

	// Check that resource was registered with correct type
	resource, exists := rm.GetResource("test-conn")
	if !exists {
		t.Error("Expected resource to be registered")
	}

	if resource.Type != ResourceTypeNetwork {
		t.Errorf("Expected resource type Network, got %s", resource.Type.String())
	}

	if resource.Description != "Database connection" {
		t.Errorf("Expected description 'Database connection', got %s", resource.Description)
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