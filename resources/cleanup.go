package resources

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// ResourceType represents the type of resource
type ResourceType int

const (
	ResourceTypeFile ResourceType = iota
	ResourceTypeNetwork
	ResourceTypeMemory
	ResourceTypeGoroutine
	ResourceTypeCustom
)

// CleanupFunc represents a cleanup function for a resource
type CleanupFunc func() error

// Resource represents a managed resource
type Resource struct {
	ID          string
	Type        ResourceType
	Description string
	Cleanup     CleanupFunc
	CreatedAt   time.Time
}

// ResourceManager manages resources and ensures proper cleanup
type ResourceManager struct {
	mu        sync.RWMutex
	resources map[string]*Resource
	stats     struct {
		totalResources     int64
		cleanedResources   int64
		failedCleanups     int64
		memoryUsage        int64
		goroutineCount     int64
		lastGC             time.Time
	}
	logger Logger
	ctx    context.Context
	cancel context.CancelFunc
}

// Logger interface for resource management logging
type Logger interface {
	Printf(format string, args ...interface{})
}

// DefaultLogger implements a simple logger
type DefaultLogger struct{}

func (l *DefaultLogger) Printf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

// ResourceManagerConfig holds configuration for resource manager
type ResourceManagerConfig struct {
	Logger          Logger
	EnableGC        bool
	GCInterval      time.Duration
	MaxMemoryMB     int64
	MaxGoroutines   int64
}

// NewResourceManager creates a new resource manager
func NewResourceManager(config ResourceManagerConfig) *ResourceManager {
	if config.Logger == nil {
		config.Logger = &DefaultLogger{}
	}

	if config.GCInterval <= 0 {
		config.GCInterval = 5 * time.Minute
	}

	ctx, cancel := context.WithCancel(context.Background())

	rm := &ResourceManager{
		resources: make(map[string]*Resource),
		logger:    config.Logger,
		ctx:       ctx,
		cancel:    cancel,
	}

	// Start background cleanup if enabled
	if config.EnableGC {
		go rm.backgroundCleanup(config.GCInterval)
	}

	return rm
}

// RegisterResource registers a resource for cleanup
func (rm *ResourceManager) RegisterResource(id string, resourceType ResourceType, description string, cleanup CleanupFunc) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	resource := &Resource{
		ID:          id,
		Type:        resourceType,
		Description: description,
		Cleanup:     cleanup,
		CreatedAt:   time.Now(),
	}

	rm.resources[id] = resource
	rm.stats.totalResources++

	rm.logger.Printf("Registered resource: %s (%s) - %s", id, resourceType.String(), description)
}

// UnregisterResource removes a resource from management and attempts cleanup
func (rm *ResourceManager) UnregisterResource(id string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	resource, exists := rm.resources[id]
	if !exists {
		return fmt.Errorf("resource %s not found", id)
	}

	var cleanupErr error
	if resource.Cleanup != nil {
		if err := resource.Cleanup(); err != nil {
			rm.stats.failedCleanups++
			cleanupErr = fmt.Errorf("cleanup failed for resource %s: %w", id, err)
			rm.logger.Printf("Cleanup failed for resource %s: %v", id, err)
		} else {
			rm.stats.cleanedResources++
			rm.logger.Printf("Successfully cleaned up resource: %s", id)
		}
	}

	delete(rm.resources, id)
	return cleanupErr
}

// CleanupResource cleans up a specific resource without unregistering it
func (rm *ResourceManager) CleanupResource(id string) error {
	rm.mu.RLock()
	resource, exists := rm.resources[id]
	rm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("resource %s not found", id)
	}

	if resource.Cleanup != nil {
		if err := resource.Cleanup(); err != nil {
			rm.stats.failedCleanups++
			rm.logger.Printf("Cleanup failed for resource %s: %v", id, err)
			return fmt.Errorf("cleanup failed for resource %s: %w", id, err)
		}

		rm.stats.cleanedResources++
		rm.logger.Printf("Successfully cleaned up resource: %s", id)
	}

	return nil
}

// CleanupAll cleans up all registered resources
func (rm *ResourceManager) CleanupAll() []error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	var errors []error

	for id, resource := range rm.resources {
		if resource.Cleanup != nil {
			if err := resource.Cleanup(); err != nil {
				rm.stats.failedCleanups++
				errMsg := fmt.Sprintf("cleanup failed for resource %s: %v", id, err)
				errors = append(errors, fmt.Errorf(errMsg))
				rm.logger.Printf(errMsg)
			} else {
				rm.stats.cleanedResources++
				rm.logger.Printf("Successfully cleaned up resource: %s", id)
			}
		}
	}

	// Clear all resources
	rm.resources = make(map[string]*Resource)

	return errors
}

// GetResource returns a resource by ID
func (rm *ResourceManager) GetResource(id string) (*Resource, bool) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	resource, exists := rm.resources[id]
	return resource, exists
}

// ListResources returns all registered resources
func (rm *ResourceManager) ListResources() []*Resource {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	resources := make([]*Resource, 0, len(rm.resources))
	for _, resource := range rm.resources {
		resources = append(resources, resource)
	}

	return resources
}

// GetStats returns resource management statistics
func (rm *ResourceManager) GetStats() map[string]interface{} {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	// Update runtime stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	rm.stats.memoryUsage = int64(m.Alloc)
	rm.stats.goroutineCount = int64(m.NumGoroutine)

	return map[string]interface{}{
		"total_resources":   rm.stats.totalResources,
		"active_resources":  int64(len(rm.resources)),
		"cleaned_resources": rm.stats.cleanedResources,
		"failed_cleanups":   rm.stats.failedCleanups,
		"memory_usage_mb":   float64(rm.stats.memoryUsage) / 1024 / 1024,
		"goroutine_count":   rm.stats.goroutineCount,
		"last_gc":          rm.stats.lastGC,
	}
}

// RunGC forces garbage collection
func (rm *ResourceManager) RunGC() {
	runtime.GC()
	rm.mu.Lock()
	rm.stats.lastGC = time.Now()
	rm.mu.Unlock()
	rm.logger.Printf("Forced garbage collection")
}

// backgroundCleanup runs periodic cleanup and GC
func (rm *ResourceManager) backgroundCleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-rm.ctx.Done():
			return
		case <-ticker.C:
			rm.performBackgroundCleanup()
		}
	}
}

// performBackgroundCleanup performs background cleanup tasks
func (rm *ResourceManager) performBackgroundCleanup() {
	// Force garbage collection
	rm.RunGC()

	// Check for long-running resources
	rm.mu.RLock()
	now := time.Now()
	var longRunningResources []*Resource
	for _, resource := range rm.resources {
		if now.Sub(resource.CreatedAt) > 30*time.Minute {
			longRunningResources = append(longRunningResources, resource)
		}
	}
	rm.mu.RUnlock()

	if len(longRunningResources) > 0 {
		rm.logger.Printf("Found %d long-running resources (>30 minutes)", len(longRunningResources))
		for _, resource := range longRunningResources {
			rm.logger.Printf("Long-running resource: %s (%s) - created at %s",
				resource.ID, resource.Type.String(), resource.CreatedAt.Format(time.RFC3339))
		}
	}
}

// Close shuts down the resource manager and cleans up all resources
func (rm *ResourceManager) Close() {
	rm.logger.Printf("Shutting down resource manager")

	// Cancel background context
	rm.cancel()

	// Cleanup all resources
	errors := rm.CleanupAll()
	if len(errors) > 0 {
		rm.logger.Printf("Encountered %d errors during cleanup", len(errors))
		for _, err := range errors {
			rm.logger.Printf("Cleanup error: %v", err)
		}
	}

	rm.logger.Printf("Resource manager shutdown complete")
}

// String returns the string representation of ResourceType
func (rt ResourceType) String() string {
	switch rt {
	case ResourceTypeFile:
		return "File"
	case ResourceTypeNetwork:
		return "Network"
	case ResourceTypeMemory:
		return "Memory"
	case ResourceTypeGoroutine:
		return "Goroutine"
	case ResourceTypeCustom:
		return "Custom"
	default:
		return "Unknown"
	}
}

// WithCleanup creates a resource with cleanup function
func WithCleanup(id string, resourceType ResourceType, description string, cleanup CleanupFunc) func(*ResourceManager) {
	return func(rm *ResourceManager) {
		rm.RegisterResource(id, resourceType, description, cleanup)
	}
}

// NewFileResource creates a file resource helper
func NewFileResource(id, filePath string, cleanup CleanupFunc) func(*ResourceManager) {
	return WithCleanup(id, ResourceTypeFile, fmt.Sprintf("File: %s", filePath), cleanup)
}

// NewNetworkResource creates a network resource helper
func NewNetworkResource(id, description string, cleanup CleanupFunc) func(*ResourceManager) {
	return WithCleanup(id, ResourceTypeNetwork, description, cleanup)
}

// NewMemoryResource creates a memory resource helper
func NewMemoryResource(id, description string, cleanup CleanupFunc) func(*ResourceManager) {
	return WithCleanup(id, ResourceTypeMemory, description, cleanup)
}

// NewCustomResource creates a custom resource helper
func NewCustomResource(id, description string, cleanup CleanupFunc) func(*ResourceManager) {
	return WithCleanup(id, ResourceTypeCustom, description, cleanup)
}